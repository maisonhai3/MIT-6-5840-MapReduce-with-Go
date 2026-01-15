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

type ExampleArgs struct {
	X int
}

type ExampleReply struct {
	Y int
}

// Add your RPC definitions here.

// TaskType represents the type of task assigned to a worker.
type TaskType int

const (
	MapTask    TaskType = iota // 0: Map task
	ReduceTask                 // 1: Reduce task
	WaitTask                   // 2: No task available, worker should wait
	ExitTask                   // 3: All tasks done, worker should exit
)

// GetTaskArgs is used by the worker to request a task from the coordinator.
type GetTaskArgs struct {
	WorkerID int // Optional: for debugging/logging purposes
}

// GetTaskReply contains the task information sent by the coordinator.
type GetTaskReply struct {
	TaskType   TaskType // Type of task (Map, Reduce, Wait, Exit)
	TaskID     int      // Unique ID for this task
	InputFile  string   // For Map: the input filename
	NReduce    int      // Total number of reduce tasks (for Map to partition)
	NMap       int      // Total number of map tasks (for Reduce to find intermediates)
	ReduceID   int      // For Reduce: which reduce partition to process
}

// ReportTaskArgs is used by the worker to report task completion.
type ReportTaskArgs struct {
	TaskType TaskType // Type of task completed (Map or Reduce)
	TaskID   int      // ID of the completed task
	Success  bool     // Whether the task completed successfully
}

// ReportTaskReply is the coordinator's response to a task report.
type ReportTaskReply struct {
	Acknowledged bool // Confirmation that the report was received
}

// Cook up a unique-ish UNIX-domain socket name
// in /var/tmp, for the coordinator.
// Can't use the current directory since
// Athena AFS doesn't support UNIX-domain sockets.
func coordinatorSock() string {
	s := "/var/tmp/5840-mr-"
	s += strconv.Itoa(os.Getuid())
	return s
}
