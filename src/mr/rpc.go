package mr

//
// RPC definitions.
//
// remember to capitalize all names.
//

import "os"
import "strconv"

//
// example to show how to declare the arguments
// and reply for an RPC.
//

type TaskRequestArgs struct {
	X int
}

type TaskReply struct {
	MapTaskNum    int
	ReduceTaskNum int
	File          string
	TaskId        int
	TaskType      TaskTypes
	Ok            bool
}

type TaskFinishedArgs struct {
	TaskType TaskTypes
	File     string
}

type TaskFinishedReply struct {
	Ok bool
}

// Add your RPC definitions here.

// Cook up a unique-ish UNIX-domain socket name
// in /var/tmp, for the master.
// Can't use the current directory since
// Athena AFS doesn't support UNIX-domain sockets.
func masterSock() string {
	s := "/var/tmp/824-mr-"
	s += strconv.Itoa(os.Getuid())
	return s
}
