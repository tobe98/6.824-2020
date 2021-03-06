package mr

import (
	"fmt"
	"log"
	"strconv"
	"sync"
	"time"
)
import "net"
import "os"
import "net/rpc"
import "net/http"

type TaskStatusMap map[string]*TaskDetail
type TaskDetail struct {
	TaskId     int
	TaskStatus int
}
type TaskTypes int

// Master 结构体，里面应该存储 Master 的所有必要信息（废话）
type Master struct {
	// map 任务状态：UnStarted, Processing, Finished
	MapTaskStatus TaskStatusMap
	MapTaskNum    int

	MapTaskFinished bool

	// Reduce Task 的个数，即 nReduce
	ReduceTaskNum int

	// reduce task 状态
	ReduceTaskStatus   TaskStatusMap
	ReduceTaskFinished bool

	// master 当前任务状态
	TaskStatus int
	RWMux      *sync.RWMutex
}

const (
	Free = iota
	MapTask
	ReduceTask
)

const (
	UnStarted = iota
	Processing
	Finished
	UnGenerated
)

// 初始化 MapTaskStatus
func InitMapTaskStatus(files []string) TaskStatusMap {
	taskStatusMap := make(TaskStatusMap)
	for i, file := range files {
		taskStatusMap[file] = &TaskDetail{
			TaskId:     i,
			TaskStatus: UnStarted,
		}
	}
	return taskStatusMap
}

// 初始化 ReduceTaskStatus
// 为了统一，也把 ReduceTask 的键改为文件名，即 "mr-out-" + string.Itoa(i)
func InitReduceTaskStatus(nReduce int) TaskStatusMap {
	taskStatusMap := make(TaskStatusMap)
	for i := 0; i < nReduce; i++ {
		filename := GetReduceFileName(i)
		taskStatusMap[filename] = &TaskDetail{
			TaskId:     i,
			TaskStatus: UnStarted,
			// 暂时不需要 UnGenerated 状态，worker 自动整合文件
			// TaskStatus: UnGenerated,
		}
	}
	return taskStatusMap
}

func GetReduceFileName(index int) string {
	return "mr-out-" + strconv.Itoa(index)
}

// Your code here -- RPC handlers for the worker to call.

//
// an example RPC handler.
//
// the RPC argument and reply types are defined in rpc.go.
//
func (master *Master) ReplyTaskToWorker(args *TaskRequestArgs, reply *TaskReply) error {
	file, taskType, taskId, ok := master.GetUnStartedTask()
	reply.ReduceTaskNum = master.ReduceTaskNum
	reply.MapTaskNum = master.MapTaskNum
	reply.TaskId = taskId
	reply.File = file
	reply.TaskType = taskType
	reply.Ok = ok
	return nil
}

// worker 完成 task 后调用该 RPC 通知 master
func (master *Master) ReportFinishedTask(args *TaskFinishedArgs, reply *TaskFinishedReply) error {
	file := args.File
	master.RWMux.Lock()
	defer master.RWMux.Unlock()

	if args.TaskType == MapTask {
		master.MapTaskStatus[file].TaskStatus = Finished
		reply.Ok = true
		fmt.Printf("The Map task %v has finished\n", file)
		go master.CheckMasterStatus()
		return nil
	} else if args.TaskType == ReduceTask {
		master.ReduceTaskStatus[file].TaskStatus = Finished
		reply.Ok = true
		fmt.Printf("The Reduce task %v has finished\n", file)
		go master.CheckMasterStatus()
		return nil
	}

	reply.Ok = false
	// TODO 应该定义一个错误
	return nil
}

// master 状态机，在收到 worker 传来的 task 任务完成信息后调用
// 更改 TaskStatus
func (master *Master) CheckMasterStatus() {
	if master.TaskStatus == MapTask {
		if master.IsPhaseFinished(MapTask) {
			master.RWMux.Lock()
			defer master.RWMux.Unlock()
			master.TaskStatus = ReduceTask
			return
		}
	} else if master.TaskStatus == ReduceTask {
		if master.IsPhaseFinished(ReduceTask) {
			master.RWMux.Lock()
			defer master.RWMux.Unlock()
			master.TaskStatus = Free
			return
		}
	}
}

// 检查某一阶段（map/reduce phase）是否结束
func (master *Master) IsPhaseFinished(taskType TaskTypes) bool {
	master.RWMux.RLock()
	defer master.RWMux.RUnlock()
	if taskType == MapTask {
		for _, v := range master.MapTaskStatus {
			if v.TaskStatus != Finished {
				return false
			}
		}
		return true
	}
	if taskType == ReduceTask {
		for _, v := range master.ReduceTaskStatus {
			if v.TaskStatus != Finished {
				return false
			}
		}
		return true
	}
	return true
}

//
// main/mrmaster.go calls Done() periodically to find out
// if the entire job has finished.
// 如果整个任务完成，返回 true
func (master *Master) Done() bool {
	// change ret := false to true so that the master exits immediately
	//ret := true
	// Your code here.

	return master.TaskStatus == Free
}

// 从 map/reduce 任务中获取一个没有开始的任务
func (master *Master) GetUnStartedTask() (string, TaskTypes, int, bool) {
	master.RWMux.Lock()
	defer master.RWMux.Unlock()
	if master.TaskStatus == MapTask {
		for fileName, taskDetail := range master.MapTaskStatus {
			if taskDetail.TaskStatus == UnStarted {
				master.MapTaskStatus[fileName].TaskStatus = Processing
				go master.TimeOutDetection(fileName, MapTask)
				return fileName, MapTask, taskDetail.TaskId, true
			}
		}
		return "", MapTask, 0, false
	}
	if master.TaskStatus == ReduceTask {
		for fileName, taskDetail := range master.ReduceTaskStatus {
			if taskDetail.TaskStatus == UnStarted {
				master.ReduceTaskStatus[fileName].TaskStatus = Processing
				go master.TimeOutDetection(fileName, ReduceTask)
				return fileName, ReduceTask, taskDetail.TaskId, true
			}
		}
		return "", ReduceTask, 0, false
	}
	return "", Free, 0, false
}

// 超时检测任务，如果 10s 后任务还是 processing 状态，就认为
// worker 故障，直接将任务设为 UnStarted 状态
//
func (master *Master) TimeOutDetection(file string, taskType int) {
	time.Sleep(5 * time.Second)
	master.RWMux.Lock()
	defer master.RWMux.Unlock()
	if taskType == MapTask {
		if master.MapTaskStatus[file].TaskStatus == Processing {
			fmt.Printf("The worker of Map %v has faild\n", file)
			master.MapTaskStatus[file].TaskStatus = UnStarted
		}
		return
	}

	if taskType == ReduceTask {
		if master.ReduceTaskStatus[file].TaskStatus == Processing {
			fmt.Printf("The worker of Reduce %v has faild\n", file)
			master.ReduceTaskStatus[file].TaskStatus = UnStarted
		}
		return
	}
}

//
// start a thread that listens for RPCs from worker.go
// 创建一个线程，监听 worker 的 RPC 请求
func (master *Master) server() {
	rpc.Register(master)
	rpc.HandleHTTP()
	//listener, err := net.Listen("tcp", ":1234")
	sockname := masterSock()
	os.Remove(sockname)
	listener, err := net.Listen("unix", sockname)
	if err != nil {
		log.Fatal("listen error:", err)
	}
	go http.Serve(listener, nil)
}

//
// create a Master.
// main/mrmaster.go calls this function.
// nReduce is the number of reduce tasks to use.
// 创建具有 n 个 reduce 任务的 master
//
func MakeMaster(files []string, nReduce int) *Master {
	// 初始化 master
	master := Master{
		MapTaskStatus:    InitMapTaskStatus(files),
		ReduceTaskNum:    nReduce,
		ReduceTaskStatus: InitReduceTaskStatus(nReduce),
		RWMux:            &sync.RWMutex{},
		TaskStatus:       MapTask,
		MapTaskNum:       len(files),
	}

	//

	master.server()
	return &master
}
