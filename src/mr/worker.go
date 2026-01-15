package mr

import (
	"encoding/json"
	"fmt"
	"hash/fnv"
	"io/ioutil"
	"log"
	"net/rpc"
	"os"
	"sort"
	"time"
)

// KeyValue is the Map functions return type.
type KeyValue struct {
	Key   string
	Value string
}

// ByKey implements sort.Interface for []KeyValue based on Key.
type ByKey []KeyValue

func (a ByKey) Len() int           { return len(a) }
func (a ByKey) Swap(i, j int)      { a[i], a[j] = a[j], a[i] }
func (a ByKey) Less(i, j int) bool { return a[i].Key < a[j].Key }

// use ihash(key) % NReduce to choose the reduce
// task number for each KeyValue emitted by Map.
func ihash(key string) int {
	h := fnv.New32a()
	h.Write([]byte(key))
	return int(h.Sum32() & 0x7fffffff)
}

// Worker is the main function called by main/mrworker.go.
// It loops forever, asking the coordinator for tasks and executing them.
func Worker(mapf func(string, string) []KeyValue,
	reducef func(string, []string) string) {

	for {
		// Request a task from the coordinator
		task := requestTask()

		switch task.TaskType {
		case MapTask:
			executeMapTask(task, mapf)
		case ReduceTask:
			executeReduceTask(task, reducef)
		case WaitTask:
			// No task available, wait and try again
			time.Sleep(100 * time.Millisecond)
		case ExitTask:
			// All tasks done, exit the worker
			return
		}
	}
}

// requestTask asks the coordinator for a new task.
func requestTask() GetTaskReply {
	args := GetTaskArgs{}
	reply := GetTaskReply{}

	ok := call("Coordinator.GetTask", &args, &reply)
	if !ok {
		// Coordinator is unreachable, assume job is done
		reply.TaskType = ExitTask
	}
	return reply
}

// reportTask reports the completion status of a task to the coordinator.
func reportTask(taskType TaskType, taskID int, success bool) {
	args := ReportTaskArgs{
		TaskType: taskType,
		TaskID:   taskID,
		Success:  success,
	}
	reply := ReportTaskReply{}

	// Ignore errors - if coordinator is gone, we'll find out on next GetTask
	call("Coordinator.ReportTask", &args, &reply)
}

// executeMapTask performs a map task.
func executeMapTask(task GetTaskReply, mapf func(string, string) []KeyValue) {
	// Read the input file
	content, err := ioutil.ReadFile(task.InputFile)
	if err != nil {
		log.Printf("cannot read %v: %v", task.InputFile, err)
		reportTask(MapTask, task.TaskID, false)
		return
	}

	// Apply the map function
	kva := mapf(task.InputFile, string(content))

	// Partition the key-value pairs into nReduce buckets
	buckets := make([][]KeyValue, task.NReduce)
	for i := range buckets {
		buckets[i] = []KeyValue{}
	}
	for _, kv := range kva {
		bucket := ihash(kv.Key) % task.NReduce
		buckets[bucket] = append(buckets[bucket], kv)
	}

	// Write each bucket to an intermediate file
	for reduceID, bucket := range buckets {
		// Create a temporary file for atomic write (in current dir to allow atomic rename)
		tempFile, err := os.CreateTemp(".", "mr-map-tmp-*")
		if err != nil {
			log.Printf("cannot create temp file: %v", err)

			reportTask(MapTask, task.TaskID, false)
			return
		}

		// Encode key-value pairs as JSON
		enc := json.NewEncoder(tempFile)
		for _, kv := range bucket {
			if err := enc.Encode(&kv); err != nil {
				log.Printf("cannot encode kv: %v", err)

				tempFile.Close()
				os.Remove(tempFile.Name())

				reportTask(MapTask, task.TaskID, false)
				return
			}
		}
		tempFile.Close()

		// Atomically rename to final filename: mr-X-Y
		finalName := fmt.Sprintf("mr-%d-%d", task.TaskID, reduceID)
		if err := os.Rename(tempFile.Name(), finalName); err != nil {
			log.Printf("cannot rename temp file: %v", err)

			os.Remove(tempFile.Name())

			reportTask(MapTask, task.TaskID, false)
			return
		}
	}

	// Report success
	reportTask(MapTask, task.TaskID, true)
}

// executeReduceTask performs a reduce task.
func executeReduceTask(task GetTaskReply, reducef func(string, []string) string) {
	// Collect all intermediate key-value pairs for this reduce partition
	var intermediate []KeyValue

	// Read from all map tasks' intermediate files for this reduce partition
	for mapID := 0; mapID < task.NMap; mapID++ {
		filename := fmt.Sprintf("mr-%d-%d", mapID, task.ReduceID)
		file, err := os.Open(filename)
		if err != nil {
			// File might not exist if map produced no output for this partition
			continue
		}

		dec := json.NewDecoder(file)
		for {
			var kv KeyValue
			if err := dec.Decode(&kv); err != nil {
				break
			}
			intermediate = append(intermediate, kv)
		}
		file.Close()
	}

	// Sort by key (required for grouping)
	sort.Sort(ByKey(intermediate))

	// Create a temporary output file (in current dir to allow atomic rename)
	tempFile, err := os.CreateTemp(".", "mr-reduce-tmp-*")
	if err != nil {
		log.Printf("cannot create temp file: %v", err)
		reportTask(ReduceTask, task.TaskID, false)
		return
	}

	// Group by key and apply reduce function
	i := 0
	for i < len(intermediate) {
		j := i + 1
		// Find all values with the same key
		for j < len(intermediate) && intermediate[j].Key == intermediate[i].Key {
			j++
		}

		// Collect values for this key
		values := []string{}
		for k := i; k < j; k++ {
			values = append(values, intermediate[k].Value)
		}

		// Apply reduce function
		output := reducef(intermediate[i].Key, values)

		// Write output in the correct format: "%v %v\n"
		fmt.Fprintf(tempFile, "%v %v\n", intermediate[i].Key, output)

		i = j
	}
	tempFile.Close()

	// Atomically rename to final filename: mr-out-X
	finalName := fmt.Sprintf("mr-out-%d", task.ReduceID)
	if err := os.Rename(tempFile.Name(), finalName); err != nil {
		log.Printf("cannot rename temp file: %v", err)
		os.Remove(tempFile.Name())
		reportTask(ReduceTask, task.TaskID, false)
		return
	}

	// Report success
	reportTask(ReduceTask, task.TaskID, true)
}

// call sends an RPC request to the coordinator, wait for the response.
// usually returns true.
// returns false if something goes wrong.
func call(rpcname string, args interface{}, reply interface{}) bool {
	sockname := coordinatorSock()
	c, err := rpc.DialHTTP("unix", sockname)
	if err != nil {
		// Don't log.Fatal here - coordinator might have exited normally
		return false
	}
	defer c.Close()

	err = c.Call(rpcname, args, reply)
	if err == nil {
		return true
	}

	fmt.Println(err)
	return false
}
