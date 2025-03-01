package agent

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"strconv"
	"sync"
	"time"
)
type TaskWrapper struct {
    Task Task `json:"task"`
}

type Task struct {
	ID            string    `json:"id"`
	Arg1          string    `json:"arg1"`          
	Arg2          string    `json:"arg2"`          
	Operation     Operation `json:"operation"`     
	OperationTime int       `json:"operation_time"`
}

type Operation string
const (
	OperationAddition       Operation = "addition"
	OperationSubtraction    Operation = "subtraction"
	OperationMultiplication Operation = "multiplication"
	OperationDivision       Operation = "division"
)

type Result struct {
	ID     string  `json:"id"`
	Result float64 `json:"result"`
}

type Agent struct {
	serverURL string
	workers   int
	wg        sync.WaitGroup
}

func NewAgent(serverURL string, workers int) *Agent {
	return &Agent{
		serverURL: serverURL,
		workers:   workers,
	}
}

func (a *Agent) fetchTask() (*Task, error) {
	resp, err := http.Get(a.serverURL + "/internal/task")
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("failed to fetch task, status: %d", resp.StatusCode)
	}

	var taskWrapper TaskWrapper
	body, _ := io.ReadAll(resp.Body)
	fmt.Println("Received raw JSON:", string(body)) 

	if err := json.Unmarshal(body, &taskWrapper); err != nil {
		return nil, err
	}

	fmt.Printf("Parsed task: %+v\n", taskWrapper.Task) 
	return &taskWrapper.Task, nil
}


func (a *Agent) sendResult(result Result) error {
	data, err := json.Marshal(result)
	if err != nil {
		return err
	}

	resp, err := http.Post(a.serverURL+"/internal/task", "application/json", bytes.NewBuffer(data))
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		
		return fmt.Errorf("failed to send result, status: %d, body: %s", resp.StatusCode, string(body))
	}

	return nil
}

func compute(task *Task) (float64, error) {
	fmt.Printf("Received operation: '%s'\n", task.Operation) 

	switch task.Operation {

	case OperationAddition:
		operand1, err1 := strconv.ParseFloat(task.Arg1, 64)
		if err1 != nil {
            return 0, err1
        }
		operand2, err2 := strconv.ParseFloat(task.Arg2, 64)
		if err2 != nil {
            return 0, err2
        }
		return operand1 + operand2, nil
	case OperationSubtraction:
		operand1, err1 := strconv.ParseFloat(task.Arg1, 64)
		if err1 != nil {
            return 0, err1
        }
		operand2, err2 := strconv.ParseFloat(task.Arg2, 64)
		if err2 != nil {
            return 0, err2
        }
		time.Sleep(5*time.Second)
		return operand1 - operand2, nil
	case OperationMultiplication:
		operand1, err1 := strconv.ParseFloat(task.Arg1, 64)
		if err1 != nil {
            return 0, err1
        }
		operand2, err2 := strconv.ParseFloat(task.Arg2, 64)
		if err2 != nil {
            return 0, err2
        }
		time.Sleep(5*time.Second)

		return operand1 * operand2, nil
	case OperationDivision:
		operand1, err1 := strconv.ParseFloat(task.Arg1, 64)
		if err1 != nil {
			return 0, err1
		}
		operand2, err2  := strconv.ParseFloat(task.Arg2, 64) 
		if err2 != nil {
            return 0, err2
        }
		if operand2 == 0 {
			return 0, fmt.Errorf("division by zero")
		}
		time.Sleep(5*time.Second)

		return operand1 / operand2, nil
		
	default:
		return 0, fmt.Errorf("unknown operation")
	}
}

func (a *Agent) worker() {
	defer a.wg.Done()
	for {
		task, err := a.fetchTask()
		if err != nil {
			log.Println("Error fetching task:", err)
			time.Sleep(2 * time.Second)
			continue
		}

		result, err := compute(task)
		if err != nil {
			log.Println("Error computing task:", err)
			continue
		}

		if err := a.sendResult(Result{ID: task.ID, Result: result}); err != nil {
			log.Println("Error sending result:", err)
		}
	}
}

func (a *Agent) Run() {
	for i := 0; i < a.workers; i++ {
		a.wg.Add(1)
		go a.worker()
	}
	a.wg.Wait()
}


