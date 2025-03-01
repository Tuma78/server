package application

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strconv"
	"strings"
	"sync"
	"github.com/Tuma78/server/models"
	"github.com/google/uuid"
)

type Config struct {
	Addr                  string
	TimeAdditionMS        int
	TimeSubtractionMS     int
	TimeMultiplicationsMS int
	TimeDivisionsMS       int
}

func ConfigFromEnv() *Config {
	config := new(Config)
	config.Addr = os.Getenv("PORT")
	if config.Addr == "" {
		config.Addr = "8080"
	}
	config.TimeAdditionMS, _ = strconv.Atoi(os.Getenv("TIME_ADDITION_MS"))
	if config.TimeAdditionMS == 0 {
		config.TimeAdditionMS = 1000
	}
	config.TimeSubtractionMS, _ = strconv.Atoi(os.Getenv("TIME_SUBTRACTION_MS"))
	if config.TimeSubtractionMS == 0 {
		config.TimeSubtractionMS = 1000
	}
	config.TimeMultiplicationsMS, _ = strconv.Atoi(os.Getenv("TIME_MULTIPLICATIONS_MS"))
	if config.TimeMultiplicationsMS == 0 {
		config.TimeMultiplicationsMS = 2000
	}
	config.TimeDivisionsMS, _ = strconv.Atoi(os.Getenv("TIME_DIVISIONS_MS"))
	if config.TimeDivisionsMS == 0 {
		config.TimeDivisionsMS = 2000
	}
	return config
}

type ExpressionStatus string

const (
	StatusPending    ExpressionStatus = "pending"
	StatusProcessing ExpressionStatus = "processing"
	StatusCompleted  ExpressionStatus = "completed"
	StatusFailed     ExpressionStatus = "failed"
)

// Expression представляет сохранённое выражение.
type Expression struct {
	ID               string           `json:"id"`
	Expression       string           `json:"-"`
	Status           ExpressionStatus `json:"status"`
	Result           *float64         `json:"result,omitempty"`
	Tasks            []*models.Task          `json:"-"` 
	CurrentTaskIndex int              `json:"-"` 
}



// Application – состояние оркестратора.
type Application struct {
	config      *Config
	expressions map[string]*Expression
	tasks       map[string]*models.Task
	taskQueue   []*models.Task // глобальная очередь задач
	mutex       sync.Mutex
}

func New() *Application {
	return &Application{
		config:      ConfigFromEnv(),
		expressions: make(map[string]*Expression),
		tasks:       make(map[string]*models.Task),
		taskQueue:   make([]*models.Task, 0),
	}
}

// RunServer регистрирует эндпоинты и запускает HTTP-сервер.
func (a *Application) RunServer() error {
	http.HandleFunc("/api/v1/calculate", a.CalcHandler)
	http.HandleFunc("/api/v1/expressions", a.ExpressionsHandler)
	http.HandleFunc("/api/v1/expressions/", a.ExpressionHandler)
	http.HandleFunc("/internal/task", a.giveTaskHandler)
	return http.ListenAndServe(":"+a.config.Addr, nil)
}

// CalcHandler принимает арифметическое выражение, разбивает его на задачи и сохраняет его.
func (a *Application) CalcHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	var req models.Request
	defer r.Body.Close()
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}
	if !isValidExpression(req.Expression) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusUnprocessableEntity)
		resp := models.Response{Error: "Expression is not valid"}
		json.NewEncoder(w).Encode(resp)
		return
	}

	exprID, err := a.addExpression(req.Expression)
	if err != nil {
		http.Error(w, "Error processing expression", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	resp := models.Response{ID: exprID}
	json.NewEncoder(w).Encode(resp)
}

// addExpression преобразует выражение в RPN, строит последовательность задач и сохраняет выражение.
func (a *Application) addExpression(exprStr string) (string, error) {
	exprID := uuid.New().String()
	tokens, err := infixToRPN(exprStr)
	if err != nil {
		return "", err
	}
	tasks, err := buildTasksFromRPN(tokens, a.config, exprID)
	if err != nil {
		return "", err	
	}
	expr := &Expression{
		ID:               exprID,
		Expression:       exprStr,
		Status:           StatusPending,
		Tasks:            tasks,
		CurrentTaskIndex: 0,
	}
	a.mutex.Lock()
	a.expressions[exprID] = expr
	for _, t := range tasks {
		a.tasks[t.ID] = t
	}
	if len(tasks) > 0 && isNumeric(tasks[0].Arg1) && isNumeric(tasks[0].Arg2) {
		a.taskQueue = append(a.taskQueue, tasks[0])
		expr.Status = StatusProcessing
	}
	a.mutex.Unlock()
	return expr.ID, nil
}

// giveTaskHandler обрабатывает GET-запрос на выдачу задачи агенту и POST-запрос с результатом выполнения.
func (a *Application) giveTaskHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodGet {
		a.mutex.Lock()
		defer a.mutex.Unlock()
		if len(a.taskQueue) == 0 {
			http.Error(w, "No task available", http.StatusNotFound)
			return
		}
		task := a.taskQueue[0]
		a.taskQueue = a.taskQueue[1:]
		outTask := struct {
			ID            string    `json:"id"`
			Arg1          string    `json:"arg1"`
			Arg2          string    `json:"arg2"`
			Operation     models.Operation `json:"operation"`
			OperationTime int       `json:"operation_time"`
		}{
			ID:            task.ID,
			Arg1:          task.Arg1,
			Arg2:          task.Arg2,
			Operation:     task.Operation,
			OperationTime: task.OperationTime,
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{"task": outTask})
		return
	}

	if r.Method == http.MethodPost {
		var req models.TaskResultRequest
		defer r.Body.Close()
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "Invalid request body", http.StatusUnprocessableEntity)
			return
		}
		a.mutex.Lock()
		_, ok := a.tasks[req.ID]
		if !ok {
			a.mutex.Unlock()
			http.Error(w, "Task not found", http.StatusNotFound)
			return
		}
		var expr *Expression
		for _, e := range a.expressions {
			for _, t := range e.Tasks {
				if t.ID == req.ID {
					expr = e
					break
				}
			}
			if expr != nil {
				break
			}
		}
		if expr == nil {
			a.mutex.Unlock()
			http.Error(w, "Expression not found", http.StatusNotFound)
			return
		}
		var completedIndex int = -1
		for i, t := range expr.Tasks {
			if t.ID == req.ID {
				completedIndex = i
				break
			}
		}
		if completedIndex == -1 || completedIndex != expr.CurrentTaskIndex {
			a.mutex.Unlock()
			http.Error(w, "Task is not the current one", http.StatusBadRequest)
			return
		}
		expr.CurrentTaskIndex++
		if expr.CurrentTaskIndex < len(expr.Tasks) {
			nextTask := expr.Tasks[expr.CurrentTaskIndex]
			placeholder := fmt.Sprintf("T%d", completedIndex)
			if nextTask.Arg1 == placeholder {
				nextTask.Arg1 = fmt.Sprintf("%f", req.Result)
			}
			if nextTask.Arg2 == placeholder {
				nextTask.Arg2 = fmt.Sprintf("%f", req.Result)
			}
			if isNumeric(nextTask.Arg1) && isNumeric(nextTask.Arg2) {
				a.taskQueue = append(a.taskQueue, nextTask)
			}
		} else {
			expr.Status = StatusCompleted
			expr.Result = &req.Result
		}
		a.mutex.Unlock()
		w.WriteHeader(http.StatusOK)
		return
	}

	http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
}

// ExpressionsHandler возвращает список всех выражений с их статусами.
func (a *Application) ExpressionsHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	type OutExpression struct {
		ID     string           `json:"id"`
		Status ExpressionStatus `json:"status"`
		Result *float64         `json:"result,omitempty"`
	}
	a.mutex.Lock()
	defer a.mutex.Unlock()
	out := make([]OutExpression, 0, len(a.expressions))
	for _, expr := range a.expressions {
		out = append(out, OutExpression{
			ID:     expr.ID,
			Status: expr.Status,
			Result: expr.Result,
		})
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{"expressions": out})
}

func (a *Application) ExpressionHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	parts := strings.Split(r.URL.Path, "/")
	if len(parts) < 4 {
		http.Error(w, "ID not provided", http.StatusBadRequest)
		return
	}
	id := parts[3]
	a.mutex.Lock()
	expr, ok := a.expressions[id]
	a.mutex.Unlock()
	if !ok {
		http.Error(w, "Expression not found", http.StatusNotFound)
		return
	}
	type OutExpression struct {
		ID     string           `json:"id"`
		Status ExpressionStatus `json:"status"`
		Result *float64         `json:"result,omitempty"`
	}
	out := OutExpression{
		ID:     expr.ID,
		Status: expr.Status,
		Result: expr.Result,
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{"expression": out})
}

func infixToRPN(expr string) ([]string, error) {
	tokens := strings.Fields(expr)
	output := []string{}
	opStack := []string{}
	precedence := map[string]int{
		"+": 1,
		"-": 1,
		"*": 2,
		"/": 2,
	}
	for _, token := range tokens {
		if isNumeric(token) {
			output = append(output, token)
		} else if token == "(" {
			opStack = append(opStack, token)
		} else if token == ")" {
			for len(opStack) > 0 && opStack[len(opStack)-1] != "(" {
				output = append(output, opStack[len(opStack)-1])
				opStack = opStack[:len(opStack)-1]
			}
			if len(opStack) == 0 {
				return nil, fmt.Errorf("mismatched parentheses")
			}
			opStack = opStack[:len(opStack)-1]
		} else if token == "+" || token == "-" || token == "*" || token == "/" {
			for len(opStack) > 0 {
				top := opStack[len(opStack)-1]
				if top == "(" {
					break
				}
				if precedence[top] >= precedence[token] {
					output = append(output, top)
					opStack = opStack[:len(opStack)-1]
				} else {
					break
				}
			}
			opStack = append(opStack, token)
		} else {
			return nil, fmt.Errorf("unknown token: %s", token)
		}
	}
	for len(opStack) > 0 {
		if opStack[len(opStack)-1] == "(" || opStack[len(opStack)-1] == ")" {
			return nil, fmt.Errorf("mismatched parentheses")
		}
		output = append(output, opStack[len(opStack)-1])
		opStack = opStack[:len(opStack)-1]
	}
	return output, nil
}

func buildTasksFromRPN(tokens []string, config *Config, exprID string) ([]*models.Task, error) {
	var tasks []*models.Task
	var stack []string 
	taskCounter := 0
	for _, token := range tokens {
		if isNumeric(token) {
			stack = append(stack, token)
		} else if token == "+" || token == "-" || token == "*" || token == "/" {
			if len(stack) < 2 {
				return nil, fmt.Errorf("invalid expression")
			}
			op2 := stack[len(stack)-1]
			op1 := stack[len(stack)-2]
			stack = stack[:len(stack)-2]
			var op models.Operation
			var opTime int
			switch token {
			case "+":
				op = models.OperationAddition
				opTime = config.TimeAdditionMS
			case "-":
				op = models.OperationSubtraction
				opTime = config.TimeSubtractionMS
			case "*":
				op = models.OperationMultiplication
				opTime = config.TimeMultiplicationsMS
			case "/":
				op = models.OperationDivision
				opTime = config.TimeDivisionsMS
			}
			task := &models.Task{
				ID:            uuid.New().String(),
				Arg1:          op1,
				Arg2:          op2,
				Operation:     op,
				OperationTime: opTime,
			}
			tasks = append(tasks, task)
			placeholder := fmt.Sprintf("T%d", taskCounter)
			stack = append(stack, placeholder)
			taskCounter++
		} else {
			return nil, fmt.Errorf("unknown token in RPN: %s", token)
		}
	}
	if len(stack) != 1 {
		return nil, fmt.Errorf("invalid expression, remaining stack: %v", stack)
	}
	return tasks, nil
}

func isValidExpression(expression string) bool {
	for _, char := range expression {
		if !isValidChar(char) {
			return false
		}
	}
	return true
}

func isValidChar(char rune) bool {
	return (char >= '0' && char <= '9') ||
		char == '+' || char == '-' ||
		char == '*' || char == '/' ||
		char == '(' || char == ')' ||
		char == '.' || char == ' '
}

func isNumeric(s string) bool {
	_, err := strconv.ParseFloat(s, 64)
	return err == nil
}
