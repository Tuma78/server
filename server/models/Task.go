package models
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
