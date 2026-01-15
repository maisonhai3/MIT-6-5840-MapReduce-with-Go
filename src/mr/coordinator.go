package mr

import (
	"log"
	"net"
	"net/http"
	"net/rpc"
	"os"
	"sync"
	"time"
)

// TaskState represents the state of a task.
type TaskState int

const (
	Idle       TaskState = iota // 0: Task not yet assigned
	InProgress                  // 1: Task assigned to a worker
	Completed                   // 2: Task finished successfully
)

// TaskInfo holds metadata about a task.
type TaskInfo struct {
	State     TaskState // Current state of the task
	StartTime time.Time // When the task was assigned (for timeout detection)
	InputFile string    // For map tasks: the input file
}

// Coordinator manages MapReduce tasks.
type Coordinator struct {
	mu sync.Mutex // Protects shared state

	// Map task tracking
	mapTasks []TaskInfo // State of each map task
	nMap     int        // Total number of map tasks
	mapDone  int        // Count of completed map tasks

	// Reduce task tracking
	reduceTasks []TaskInfo // State of each reduce task
	nReduce     int        // Total number of reduce tasks
	reduceDone  int        // Count of completed reduce tasks

	// Overall state
	allDone bool // True when all tasks are complete
}

// GetTask is an RPC handler that assigns a task to a worker.
func (c *Coordinator) GetTask(args *GetTaskArgs, reply *GetTaskReply) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	// If all tasks are done, tell worker to exit
	if c.allDone {
		reply.TaskType = ExitTask
		return nil
	}

	// Phase 1: Assign Map tasks
	if c.mapDone < c.nMap {
		// Look for an idle or timed-out map task
		for i := 0; i < c.nMap; i++ {
			if c.mapTasks[i].State == Idle {
				c.assignMapTask(i, reply)
				return nil
			}
			// Check for timed-out tasks (10 seconds)
			if c.mapTasks[i].State == InProgress &&
				time.Since(c.mapTasks[i].StartTime) > 10*time.Second {
				c.assignMapTask(i, reply)
				return nil
			}
		}
		// All map tasks are in progress, tell worker to wait
		reply.TaskType = WaitTask
		return nil
	}

	// Phase 2: Assign Reduce tasks (only after all maps are done)
	if c.reduceDone < c.nReduce {
		// Look for an idle or timed-out reduce task
		for i := 0; i < c.nReduce; i++ {
			if c.reduceTasks[i].State == Idle {
				c.assignReduceTask(i, reply)
				return nil
			}
			// Check for timed-out tasks (10 seconds)
			if c.reduceTasks[i].State == InProgress &&
				time.Since(c.reduceTasks[i].StartTime) > 10*time.Second {
				c.assignReduceTask(i, reply)
				return nil
			}
		}
		// All reduce tasks are in progress, tell worker to wait
		reply.TaskType = WaitTask
		return nil
	}

	// All tasks completed
	reply.TaskType = ExitTask
	c.allDone = true
	return nil
}

// assignMapTask assigns a map task to a worker.
// Caller must hold c.mu.
func (c *Coordinator) assignMapTask(taskID int, reply *GetTaskReply) {
	c.mapTasks[taskID].State = InProgress
	c.mapTasks[taskID].StartTime = time.Now()

	reply.TaskType = MapTask
	reply.TaskID = taskID
	reply.InputFile = c.mapTasks[taskID].InputFile
	reply.NReduce = c.nReduce
	reply.NMap = c.nMap
}

// assignReduceTask assigns a reduce task to a worker.
// Caller must hold c.mu.
func (c *Coordinator) assignReduceTask(taskID int, reply *GetTaskReply) {
	c.reduceTasks[taskID].State = InProgress
	c.reduceTasks[taskID].StartTime = time.Now()

	reply.TaskType = ReduceTask
	reply.TaskID = taskID
	reply.ReduceID = taskID
	reply.NReduce = c.nReduce
	reply.NMap = c.nMap
}

// ReportTask is an RPC handler that receives task completion reports.
func (c *Coordinator) ReportTask(args *ReportTaskArgs, reply *ReportTaskReply) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	reply.Acknowledged = true

	if !args.Success {
		// Task failed, mark as idle so it can be reassigned
		if args.TaskType == MapTask && args.TaskID < c.nMap {
			c.mapTasks[args.TaskID].State = Idle
		} else if args.TaskType == ReduceTask && args.TaskID < c.nReduce {
			c.reduceTasks[args.TaskID].State = Idle
		}
		return nil
	}

	// Task succeeded
	if args.TaskType == MapTask {
		if args.TaskID < c.nMap && c.mapTasks[args.TaskID].State == InProgress {
			c.mapTasks[args.TaskID].State = Completed
			c.mapDone++
		}
	} else if args.TaskType == ReduceTask {
		if args.TaskID < c.nReduce && c.reduceTasks[args.TaskID].State == InProgress {
			c.reduceTasks[args.TaskID].State = Completed
			c.reduceDone++
			// Check if all reduce tasks are done
			if c.reduceDone == c.nReduce {
				c.allDone = true
			}
		}
	}

	return nil
}

// server starts a thread that listens for RPCs from workers.
func (c *Coordinator) server() {
	rpc.Register(c)
	rpc.HandleHTTP()
	sockname := coordinatorSock()
	os.Remove(sockname)
	l, e := net.Listen("unix", sockname)
	if e != nil {
		log.Fatal("listen error:", e)
	}
	go http.Serve(l, nil)
}

// Done is called periodically by main/mrcoordinator.go to check
// if the entire job has finished.
func (c *Coordinator) Done() bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.allDone
}

// MakeCoordinator creates a Coordinator.
// main/mrcoordinator.go calls this function.
// nReduce is the number of reduce tasks to use.
func MakeCoordinator(files []string, nReduce int) *Coordinator {
	c := Coordinator{
		nMap:        len(files),
		nReduce:     nReduce,
		mapTasks:    make([]TaskInfo, len(files)),
		reduceTasks: make([]TaskInfo, nReduce),
		mapDone:     0,
		reduceDone:  0,
		allDone:     false,
	}

	// Initialize map tasks with input files
	for i, file := range files {
		c.mapTasks[i] = TaskInfo{
			State:     Idle,
			InputFile: file,
		}
	}

	// Initialize reduce tasks
	for i := 0; i < nReduce; i++ {
		c.reduceTasks[i] = TaskInfo{
			State: Idle,
		}
	}

	c.server()
	return &c
}
