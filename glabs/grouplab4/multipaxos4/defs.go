package multipaxos4

import (
	"fmt"

	"github.com/uis-dat520-s18/glabs/grouplab4/bank4"
)

// Type definitions - DO NOT EDIT

// SlotID represents a identifier for a Paxos consensus instance.
type SlotID int

// Round represents a Paxos round number.
type Round int

// NoRound is a constant that represents no specific round. It should be used
// as the value for the Vrnd field in a PromiseSlot to indicate that an
// acceptor has not voted in any previous round for this slot.
const NoRound Round = -1

// Value represents a value that can be chosen using the Paxos algorithm and
// has the following fields:
//
// ClientID: Unique identifier for the client that sent the command.
//
// ClientSeq: Client local sequence number.
//
// Noop: Boolen to indicate if this Value should be treated as a no-op.
//
// AccountNr: The account number for the account to which the transaction Txn
// should be applied.
//
// Txn: The bank4 transaction that should be applied to account with AccountNr.
type Value struct {
	ClientID      string            `json:"clientID"`
	ClientSeq     int               `json:"clientSeq"`
	Noop          bool              `json:"noop,omitempty"`
	AccountNum    int               `json:"accountNum,omitempty"`
	Txn           bank4.Transaction `json:"txn,omitempty"`
	CreateUser    bool              `json:"createUser,omitempty"`
	User          bank4.User        `json:"user,omitempty"`
	CreateAccount bool              `json:"createAccount,omitempty"`
	Account       bank4.Account     `json:"account,omitempty"`
}

// String returns a string representation of value v.
func (v Value) String() string {
	if v.Noop == true {
		return fmt.Sprintf("No-op value")
	}
	return fmt.Sprintf("Value{ClientID: %s, ClientSeq: %d, Noop: %v, AccountNr: %d, Txn: %v, CreateUser: %v,  User: %v,  CreateAccount: %v, Accout: %v}",
		v.ClientID, v.ClientSeq, v.Noop, v.AccountNum, v.Txn, v.CreateUser, v.User, v.CreateAccount, v.Account)
}

// Response represents a reponse that can be chosen using the Paxos algorithm and
// has the following fields:
//
// ClientID: Unique identifier for the client that sent the command.
//
// ClientSeq: Client local sequence number.
//
// TxnRes: The transaction result.
type Response struct {
	ClientID  string                  `json:"clientID"`
	ClientSeq int                     `json:"clientSeq"`
	TxnRes    bank4.TransactionResult `json:"txnRes,omitempty"`
	User      bank4.User              `json:"user,omitempty"`
	Account   bank4.Account           `json:"account,omitempty"`
	Error     string                  `json:"error,omitempty"`
}

// String returns a string representation of response r.
func (r Response) String() string {
	return fmt.Sprintf("Response{ClientID: %s, ClientSeq: %d, TxnRes: %v, User: %v, Account: %v, Error: %v}",
		r.ClientID, r.ClientSeq, r.TxnRes, r.User, r.Account, r.Error)
}

// Message definitions - DO NOT EDIT

// Prepare represents a Multi-Paxos prepare message.
type Prepare struct {
	From int
	Slot SlotID
	Crnd Round
}

// Promise represents a Multi-Paxos promise message.
type Promise struct {
	To, From int
	Rnd      Round
	Slots    []PromiseSlot
}

// Accept represents a Multi-Paxos accept message.
type Accept struct {
	From int
	Slot SlotID
	Rnd  Round
	Val  Value
}

// Learn represents a Multi-Paxos learn message.
type Learn struct {
	From int
	Slot SlotID
	Rnd  Round
	Val  Value
}

// String returns a string representation of prepare p.
func (p Prepare) String() string {
	return fmt.Sprintf("Prepare{From: %d, Slot: %d, Crnd: %d}", p.From, p.Slot, p.Crnd)
}

// String returns a string representation of promise p.
func (p Promise) String() string {
	if p.Slots == nil {
		return fmt.Sprintf("Promise{To: %d, From: %d, Rnd: %d, No values reported (nil slice)}",
			p.To, p.From, p.Rnd)
	}
	if len(p.Slots) == 0 {
		return fmt.Sprintf("Promise{To: %d, From: %d, Rnd: %d, No values reported (empty slice)}",
			p.To, p.From, p.Rnd)
	}
	return fmt.Sprintf("Promise{To: %d, From: %d, Rnd: %d, Slots: %v}",
		p.To, p.From, p.Rnd, p.Slots)
}

// String returns a string representation of accept a.
func (a Accept) String() string {
	return fmt.Sprintf("Accept{From: %d, Slot: %d,  Rnd: %d, Val: %v}", a.From, a.Slot, a.Rnd, a.Val)
}

// String returns a string representation of learn l.
func (l Learn) String() string {
	return fmt.Sprintf("Learn{From: %d, Slot: %d, Rnd: %d, Val: %v}", l.From, l.Slot, l.Rnd, l.Val)
}

// PromiseSlot represents information about what round and value (if any)
// an acceptor has voted for in slot with id ID.
type PromiseSlot struct {
	ID   SlotID
	Vrnd Round
	Vval Value
}

// DecidedValue represents a value decided for a specific slot.
type DecidedValue struct {
	SlotID SlotID `json:"slotID"`
	Value  Value  `json:"value"`
}

// Testing utilities - DO NOT EDIT

var (
	testingValueOne = Value{
		ClientID:   "1234",
		ClientSeq:  42,
		AccountNum: 3,
		Txn:        bank4.Transaction{Op: bank4.Balance},
	}
	testingValueTwo = Value{
		ClientID:   "5678",
		ClientSeq:  99,
		AccountNum: 5,
		Txn:        bank4.Transaction{Op: bank4.Deposit, Amount: 1000},
	}
	testingValueThree = Value{
		ClientID:   "1369",
		ClientSeq:  4,
		AccountNum: 7,
		Txn:        bank4.Transaction{Op: bank4.Withdrawal, Amount: 1000},
	}
)
