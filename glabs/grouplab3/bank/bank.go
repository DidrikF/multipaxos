package bank

import "fmt"

// Operation represents a transaction type.
type Operation int

const (
	Balance    Operation = iota + 1 //1
	Deposit                         //2
	Withdrawal                      //3
)

var ops = [...]string{ //Array literal with length inferred
	"Balance",
	"Deposit",
	"Withdrawal",
}

// String returns a string representation of Operation o.
func (o Operation) String() string { //Balance, Deposit, Withdrawal
	if o < 1 || o > 3 {
		return fmt.Sprintf("Unknow operation (%d)", o)
	}
	return ops[o-1]
}

// Transaction represents a bank transaction with an operation type and amount.
// If Op == Balance then the Amount field should be ignored.
type Transaction struct {
	Op     Operation //int
	Amount int
}

// String returns a string representation of Transaction t.
func (t Transaction) String() string {
	switch t.Op {
	case Balance: //0
		return "Balance transaction"
	case Deposit, Withdrawal: //1, 2
		return fmt.Sprintf("%s transaction with amount %d NOK", t.Op, t.Amount)
	default:
		return fmt.Sprintf("Unknown transaction type (%d)", t.Op)
	}
}

// TransactionResult represents a result of processing a Transaction for an
// account with AcccountNum. Any error processing the transaction is reported
// in the ErrorString field. If an error is reported then the Balance field
// should be ignored.
type TransactionResult struct {
	AccountNum  int //Account.Number
	Balance     int
	ErrorString string
}

// String returns a string representation of TransactionResult tr.
func (tr TransactionResult) String() string {
	if tr.ErrorString == "" {
		return fmt.Sprintf("Transaction OK. Account: %d. Balance: %d NOK", tr.AccountNum, tr.Balance)
	}
	return fmt.Sprintf("Transaction error for account %d. Error: %s", tr.AccountNum, tr.ErrorString)
}

// Account represents a bank account with an account number and balance.
type Account struct { //these are the entities we need to store
	Number  int
	Balance int
}

// Process applies transaction txn to Account a and returns a corresponding
// TransactionResult.
func (a *Account) Process(txn Transaction) TransactionResult {
	var err string
	switch txn.Op {
	case Balance: //dont to anything
	case Deposit:
		if txn.Amount < 0 {
			err = fmt.Sprintf("Can't deposit negative amount (%d NOK)", txn.Amount)
			break
		}
		a.Balance += txn.Amount //simple addition
	case Withdrawal:
		if a.Balance-txn.Amount >= 0 {
			a.Balance -= txn.Amount
			break
		}
		err = fmt.Sprintf("Not enough funds for withdrawal. Balance: %d NOK - Requested %d NOK", a.Balance, txn.Amount)
	default:
		err = fmt.Sprintf("%v", txn) //error is the transaction if it was not processed
	}
	return TransactionResult{ //also processing a Balance "request" results in a TransactionResult
		a.Number,
		a.Balance,
		err,
	}
}

// String returns a string representation of Account a.
func (a Account) String() string {
	return fmt.Sprintf("Account %d: %d NOK", a.Number, a.Balance)
}
