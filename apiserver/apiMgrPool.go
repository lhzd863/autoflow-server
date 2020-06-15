package apiserver

import (
	"sync"
        "fmt"
        "time"
        "encoding/json"
        "strconv"

        "golang.org/x/net/context"
	"google.golang.org/grpc"

	"github.com/lhzd863/autoflow/internal/glog"
	"github.com/lhzd863/autoflow/internal/module"
        "github.com/lhzd863/autoflow/internal/util"
        "github.com/lhzd863/autoflow/internal/gproto"
)

type MgrPool struct {
	sync.Mutex
}

func NewMgrPool() *MgrPool {
	return &MgrPool{}
}

func (mp *MgrPool) JobPool() {
	for {
		glog.Glog(LogF, fmt.Sprintf("Check Job Pool..."))
		url := fmt.Sprintf("http://%v:%v/api/v1/job/pool/ls?accesstoken=%v", conf.ApiServerIp, conf.ApiServerPort, conf.AccessToken)
		jsonstr, err := util.Api_RequestPost(url, "{}")
		if err != nil {
			glog.Glog(LogF, fmt.Sprint(err))
			time.Sleep(time.Duration(10) * time.Second)
			continue
		}
		retbn := new(module.RetBean)
		err = json.Unmarshal([]byte(jsonstr), &retbn)
		if err != nil {
			glog.Glog(LogF, fmt.Sprint(err))
			time.Sleep(time.Duration(10) * time.Second)
			continue
		}
		if retbn.Status_Code != 200 {
			glog.Glog(LogF, fmt.Sprintf("post url return status code:%v", retbn.Status_Code))
			time.Sleep(time.Duration(10) * time.Second)
			continue
		}
		if retbn.Data == nil {
			glog.Glog(LogF, fmt.Sprint("get pending status job err."))
			time.Sleep(time.Duration(10) * time.Second)
			continue
		}
		retarr := (retbn.Data).([]interface{})
		serverlst := mp.ObtSlvRunningJobCnt()
		for i := 0; i < len(retarr); i++ {
			v := retarr[i].(map[string]interface{})
			s := mp.ObtJobServer(serverlst, v["server"].(string))
			if len(s) < 1 {
				glog.Glog(LogF, fmt.Sprintf("%v, %v, %v job no free server running,wait for next time.", v["flowid"], v["sys"], v["job"]))
				time.Sleep(time.Duration(10) * time.Second)
				continue
			}
			mp.SubmitJob(v, s[0])
		}
		if len(retarr) == 0 {
			glog.Glog(LogF, fmt.Sprint("job pool empty ,no running job ."))
		} else {
			glog.Glog(LogF, fmt.Sprintf("do with job cnt %v", len(retarr)))
		}
		time.Sleep(time.Duration(10) * time.Second)
	}
}

func (mp *MgrPool) SubmitJob(jobstr interface{}, slvstr interface{}) bool {
	s := slvstr.(map[string]interface{})
	jp := jobstr.(map[string]interface{})
	glog.Glog(LogF, fmt.Sprintf("%v update %v %v job status %v.", jp["flowid"], jp["sys"], jp["job"], util.STATUS_AUTO_GO))
	url0 := fmt.Sprintf("http://%v:%v/api/v1/flow/job/status/update/go?accesstoken=%v", conf.ApiServerIp, conf.ApiServerPort, conf.AccessToken)
	m := new(module.MetaParaFlowJobStatusUpdateGoBean)
	m.FlowId = jp["flowid"].(string)
	m.Sys = jp["sys"].(string)
	m.Job = jp["job"].(string)
	m.Status = util.STATUS_AUTO_GO
	m.SServer = s["workerid"].(string)
	m.Ip = s["ip"].(string)
	m.Port = s["port"].(string)
	jsonstr0, err := json.Marshal(m)
	if err != nil {
		glog.Glog(LogF, fmt.Sprint(err))
		return false
	}
	jsonstr, err := util.Api_RequestPost(url0, string(jsonstr0))
	if err != nil {
		glog.Glog(LogF, fmt.Sprint(err))
		return false
	}
	retbn := new(module.RetBean)
	err = json.Unmarshal([]byte(jsonstr), &retbn)
	if err != nil {
		glog.Glog(LogF, fmt.Sprint(err))
		return false
	}
	if retbn.Status_Code != 200 {
		glog.Glog(LogF, fmt.Sprintf("post url %v return status code:%v,msg: %v", url0, retbn.Status_Code, retbn.Status_Txt))
		return false
	}

	glog.Glog(LogF, fmt.Sprintf("rm from job pool %v,%v.%v.", jp["flowid"],jp["sys"], jp["job"]))
	url := fmt.Sprintf("http://%v:%v/api/v1/job/pool/rm?accesstoken=%v", conf.ApiServerIp, conf.ApiServerPort, conf.AccessToken)
	para := fmt.Sprintf("{\"sys\":\"%v\",\"job\":\"%v\",\"flowid\":\"%v\"}", jp["sys"], jp["job"], jp["flowid"])
	jsonstr1, err := util.Api_RequestPost(url, para)
	if err != nil {
		glog.Glog(LogF, fmt.Sprint(err))
		return false
	}
	retbn1 := new(module.RetBean)
	err = json.Unmarshal([]byte(jsonstr1), &retbn1)
	if err != nil {
		glog.Glog(LogF, fmt.Sprint(err))
		return false
	}
	if retbn1.Status_Code != 200 {
		glog.Glog(LogF, fmt.Sprintf("post url %v return status code:%v", url, retbn1.Status_Txt))
		return false
	}

	return true
}

func (mp *MgrPool) ObtSlvRunningJobCnt() []interface{} {
	url := fmt.Sprintf("http://%v:%v/api/v1/worker/cnt/exec?accesstoken=%v", conf.ApiServerIp, conf.ApiServerPort, conf.AccessToken)
	jsonstr, err := util.Api_RequestPost(url, "{}")
	if err != nil {
		glog.Glog(LogF, fmt.Sprint(err))
		return nil
	}
	retbn := new(module.RetBean)
	err = json.Unmarshal([]byte(jsonstr), &retbn)
	if err != nil {
		glog.Glog(LogF, fmt.Sprint(err))
		return nil
	}
	if retbn.Status_Code != 200 {
		glog.Glog(LogF, fmt.Sprintf("post url return status code:%v", retbn.Status_Txt))
		return nil
	}
	if retbn.Data == nil {
		glog.Glog(LogF, fmt.Sprint("get pending status job err."))
		return nil
	}
	retarr := (retbn.Data).([]interface{})
	return retarr
}

func (mp *MgrPool) ObtJobServer(arr []interface{}, slvid string) []interface{} {
	retlst := make([]interface{}, 0)
	for i := 0; i < len(arr); i++ {
		v := arr[i].(map[string]interface{})
		maxcnt, err := strconv.Atoi(v["maxcnt"].(string))
		if err != nil {
			glog.Glog(LogF, fmt.Sprintf("string conv int %v err.%v", v["maxcnt"], err))
			continue
		}
		runningcnt, err := strconv.Atoi(v["runningcnt"].(string))
		if err != nil {
			glog.Glog(LogF, fmt.Sprintf("string conv int %v err.%v", v["runningcnt"], err))
			continue
		}
		currentcnt, err := strconv.Atoi(v["currentcnt"].(string))
		if err != nil {
			glog.Glog(LogF, fmt.Sprintf("string conv int %v err.%v", v["currentcnt"], err))
			continue
		}
		if len(slvid) > 0 {
			if v["slaveid"] == slvid {
				retlst = make([]interface{}, 0)
				retlst = append(retlst, arr[i])
				break
			}
		} else if 0 < (maxcnt - runningcnt - currentcnt) {
			retlst = append(retlst, arr[i])
		}
	}
	return retlst
}

func (mp *MgrPool) ServerRoutineStatus() {
	for {
		url := fmt.Sprintf("http://%v:%v/api/v1/mst/heart/ls?accesstoken=%v", conf.ApiServerIp, conf.ApiServerPort, conf.AccessToken)
		jsonstr, err := util.Api_RequestPost(url, "{}")
		if err != nil {
			glog.Glog(LogF, fmt.Sprint(err))
			time.Sleep(time.Duration(10) * time.Second)
			continue
		}
		retbn := new(module.RetBean)
		err = json.Unmarshal([]byte(jsonstr), &retbn)
		if err != nil {
			glog.Glog(LogF, fmt.Sprint(err))
			time.Sleep(time.Duration(10) * time.Second)
			continue
		}
		if retbn.Status_Code != 200 {
			glog.Glog(LogF, fmt.Sprintf("post url return status code:%v", retbn.Status_Code))
			time.Sleep(time.Duration(10) * time.Second)
			continue
		}
		if retbn.Data == nil {
			glog.Glog(LogF, fmt.Sprint("get pending status job err."))
			time.Sleep(time.Duration(10) * time.Second)
			continue
		}
		mharr := (retbn.Data).([]module.MetaMstHeartBean)
		for _, mh := range mharr {
			// 建立连接到gRPC服务
			conn, err := grpc.Dial(mh.Ip+":"+mh.Port, grpc.WithInsecure())
			if err != nil {
				glog.Glog(LogF, fmt.Sprintf("did not connect: %v", err))
				continue
			}
			// 函数结束时关闭连接
			defer conn.Close()

			// 创建Waiter服务的客户端
			t := gproto.NewFlowMasterClient(conn)

			mifb := new(module.MetaInstanceFlowBean)
			jsonstr, err := json.Marshal(mifb)
			if err != nil {
				glog.Glog(LogF, fmt.Sprintf("json marshal %v", err))
				continue
			}
			// 调用gRPC接口
			tr, err := t.FlowStatus(context.Background(), &gproto.Req{JsonStr: string(jsonstr)})
			if err != nil {
				glog.Glog(LogF, fmt.Sprintf("could not greet: %v", err))
				continue
			}
			//change job status
			if tr.Status_Code != 200 {
				glog.Glog(LogF, fmt.Sprint(tr.Status_Txt))
				continue
			}
			if len(tr.Data) == 0 {
				glog.Glog(LogF, fmt.Sprint(tr.Status_Txt))
				continue
			}
			mf := new([]module.MetaFlowStatusBean)
			err = json.Unmarshal([]byte(tr.Data), &mf)
			if err != nil {
				glog.Glog(LogF, fmt.Sprint(err))
			}
			for _, v := range *mf {
				glog.Glog(LogF, fmt.Sprint(v))
			}
		}
		time.Sleep(time.Duration(10) * time.Second)
	}
}
