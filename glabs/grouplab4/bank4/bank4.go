package bank4

import "fmt"

// Operation represents a transaction type.
type Operation int

type User struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

const (
	Balance    Operation = iota // + 1 //1
	Deposit                     //2
	Withdrawal                  //3
	Transfer
)

var ops = [...]string{ //Array literal with length inferred
	"Balance",
	"Deposit",
	"Withdrawal",
	"transfer",
}

// String returns a string representation of Operation o.
func (o Operation) String() string { //Balance, Deposit, Withdrawal
	if o < 0 || o > 3 {
		return fmt.Sprintf("Unknow operation (%d)", o)
	}
	return ops[o-1]
}

// Transaction represents a bank transaction with an operation type and amount.
// If Op == Balance then the Amount field should be ignored.
type Transaction struct {
	Op       Operation `json:"op,omitempty"`
	Amount   int       `json:"amount,omitempty"`
	Receiver int       `json:"receiver,omitempty"`
}

// String returns a string representation of Transaction t.
func (t Transaction) String() string {
	switch t.Op {
	case Balance: //0
		return "Balance transaction"
	case Deposit, Withdrawal: //1, 2
		return fmt.Sprintf("%s transaction with amount %d NOK", t.Op, t.Amount)
	case Transfer:
		return fmt.Sprintf("%s transaction with amount %d NOK to account %d ", t.Op, t.Amount, t.Receiver)
	default:
		return fmt.Sprintf("Unknown transaction type (%d)", t.Op)
	}
}

// TransactionResult represents a result of processing a Transaction for an
// account with AcccountNum. Any error processing the transaction is reported
// in the ErrorString field. If an error is reported then the Balance field
// should be ignored.
type TransactionResult struct {
	AccountNum         int    `json:"accountNum"`
	Balance            int    `json:"balance,omitempty"`
	ReceiverAccountNum int    `json:"receiverAccountNum,omitempty"`
	ReceiverBalance    int    `json:"receiverBalance,omitempty"`
	ErrorString        string `json:"errorString,omitempty"`
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
	Number  int    `json:"number"`
	Balance int    `json:"balance,omitempty"`
	Name    string `json:"name,omitempty"`
	Owner   User   `json:"owner,omitempty"`
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
		AccountNum:  a.Number,
		Balance:     a.Balance,
		ErrorString: err,
	}

}

func (a *Account) ProcessTransfer(txn Transaction, receiverAccount Account) (TransactionResult, TransactionResult, Account) {
	var err string

	// Abort if trying to transfer a negative amount
	if txn.Amount < 0 {
		err = fmt.Sprint("Can't transfer a negative amount (%d NOK)", txn.Amount)
		return TransactionResult{ //also processing a Balance "request" results in a TransactionResult
			AccountNum:  a.Number,
			Balance:     a.Balance,
			ErrorString: err,
		}, TransactionResult{}, receiverAccount
	}
	// Abort if not enough funds on senders account
	if a.Balance-txn.Amount < 0 {
		err = fmt.Sprintf("Not enough funds for transfer. Balance: %d NOK - Requested %d NOK", a.Balance, txn.Amount)
		return TransactionResult{ //also processing a Balance "request" results in a TransactionResult
			AccountNum:  a.Number,
			Balance:     a.Balance,
			ErrorString: err,
		}, TransactionResult{}, receiverAccount
	}

	// Preform the transfer
	a.Balance -= txn.Amount
	receiverAccount.Balance += txn.Amount

	resToSender := TransactionResult{ //also processing a Balance "request" results in a TransactionResult
		AccountNum: a.Number,
		Balance:    a.Balance,
	}
	resToReceiver := TransactionResult{
		AccountNum: receiverAccount.Number,
		Balance:    receiverAccount.Balance,
	}

	// If owner of sending account is also owner of recipient
	if receiverAccount.Owner.Username == a.Owner.Username {
		resToSender.ReceiverAccountNum = receiverAccount.Number
		resToSender.ReceiverBalance = receiverAccount.Balance
	}
	return resToSender, resToReceiver, receiverAccount
}

// String returns a string representation of Account a.
func (a Account) String() string {
	return fmt.Sprintf("Account %d: Name: %s, Owner: %s, Balance: %d NOK", a.Number, a.Name, a.Owner.Username, a.Balance)
}
