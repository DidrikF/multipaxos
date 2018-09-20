package bankmanager

import (
	"fmt"

	"github.com/uis-dat520-s18/glabs/grouplab3/bank"
	"github.com/uis-dat520-s18/glabs/grouplab3/logger2"
	"github.com/uis-dat520-s18/glabs/grouplab3/multipaxos"
)

type BankManager struct {
	l                  *logger2.Logger
	adu                int
	decidedValueBuffer []multipaxos.DecidedValue
	bankAccounts       map[int]bank.Account
	responseOut        chan<- multipaxos.Response
	proposer           *multipaxos.Proposer

	DecidedValueHistory      []multipaxos.Value
	TransactionResultHistory []bank.TransactionResult
}

func NewBankManager(resOut chan<- multipaxos.Response, prop *multipaxos.Proposer, logger *logger2.Logger) *BankManager {
	bankManager := &BankManager{
		l:                        logger,
		adu:                      -1, //Keep track of the id of highest decided slot.
		decidedValueBuffer:       []multipaxos.DecidedValue{},
		bankAccounts:             map[int]bank.Account{},
		responseOut:              resOut,
		proposer:                 prop,
		DecidedValueHistory:      []multipaxos.Value{},
		TransactionResultHistory: []bank.TransactionResult{},
	}
	return bankManager
}

func (bm *BankManager) HandleDecidedValue(decidedValue multipaxos.DecidedValue) {
	fmt.Printf("#BANK MANAGER: decidedValue: %v, adu: %v, decidedValueBuffer: %v\n", decidedValue, bm.adu, bm.decidedValueBuffer)

	if int(decidedValue.SlotID) <= bm.adu {
		fmt.Println("#BANK MANAGER: returning to avoid applying the same transaction twise!")
		return
	}
	//Record decided value history
	bm.DecidedValueHistory = append(bm.DecidedValueHistory, decidedValue.Value)

	if int(decidedValue.SlotID) > bm.adu+1 {
		bm.decidedValueBuffer = append(bm.decidedValueBuffer, decidedValue)
		return
	}
	// At this point we know that decidedValue.SlotID == bm.adu+1
	if decidedValue.Value.Noop == "false" {
		accNum := decidedValue.Value.AccountNum
		if _, ok := bm.bankAccounts[accNum]; ok != true {
			bm.bankAccounts[accNum] = bank.Account{Number: accNum, Balance: 0}
			bm.l.Log(logger2.LogMessage{D: "system", S: "debug", M: fmt.Sprintf("#Bank Manager: created account %v", accNum)})
		}
		// At this point we know that the bank account exists
		account := bm.bankAccounts[accNum]
		transactionResult := account.Process(decidedValue.Value.Txn) //deposit even if a balance...
		bm.TransactionResultHistory = append(bm.TransactionResultHistory, transactionResult)
		bm.l.Log(logger2.LogMessage{D: "system", S: "debug", M: fmt.Sprintf("#Bank Manager: Processed transaction with transaction result: %v", transactionResult)})
		bm.bankAccounts[accNum] = account

		response := multipaxos.Response{
			ClientID:  decidedValue.Value.ClientID,
			ClientSeq: decidedValue.Value.ClientSeq,
			TxnRes:    transactionResult,
		}
		bm.responseOut <- response
	}
	bm.adu++
	bm.proposer.IncrementAllDecidedUpTo()
	if nextDecidedValue, ok := bm.hasBufferedValueForNextSlot(); ok == true {
		bm.HandleDecidedValue(nextDecidedValue)
	}
	return
}

func (bm *BankManager) hasBufferedValueForNextSlot() (dv multipaxos.DecidedValue, ok bool) {
	for index, decidedValue := range bm.decidedValueBuffer {
		if int(decidedValue.SlotID) == bm.adu+1 {
			bm.decidedValueBuffer = append(bm.decidedValueBuffer[:index], bm.decidedValueBuffer[index+1:]...)
			return decidedValue, true
		}
	}
	return dv, false
}

func (bm *BankManager) GetStatus() string {
	bankAccounts := ""
	sortBankAccounts := []bank.Account{}
	for _, row := range bm.bankAccounts {
		sortBankAccounts = append(sortBankAccounts, row)
	}
	sortBankAccounts = MergeSort(sortBankAccounts)
	for _, account := range sortBankAccounts {
		bankAccounts += fmt.Sprintf("%v\n", account)
	}
	return fmt.Sprintf("#Bank Manager Status: \n - Bank Accounts: \n%v", bankAccounts)
}

func (bm *BankManager) GetHistory() string {
	dvHistory := ""
	trHistory := ""
	for index, decidedVal := range bm.DecidedValueHistory {
		dvHistory += fmt.Sprintf("%v: %v\n", index, decidedVal)
	}
	for index, transactionRes := range bm.TransactionResultHistory {
		trHistory += fmt.Sprintf("%v: %v\n", index, transactionRes)
	}
	return fmt.Sprintf("#Bank Manager:\n - Decided Value History:\n%v - Transaction Result History:\n%v", dvHistory, trHistory)
}

/*
	Proposer has a IncrementAllDecidedUpTo method that should be used to indicate that a slot has been reported decided by the Learner.
	An outside caller must call this method when slots are decided in consequtive order (and wait if any gaps).

	A learner may, as specified previously, deliver decided values out of order. The server module therefore needs to ensure that only decided values
	(transactions) are processed in order. The server needs to keep track of the id for the highest decided slot to accomplish this.
	adu = -1 to begin with
	buffer out of order decided values


	handleDecidedValue(value):
		if slot id for value is larger than adu+1:
			buffer value
			return
		if value is not a no-op:
			// It should for a no-op only increment its adu and call the IncrementAllDecidedUpTo() method on the Propposer.
			if account for account number in value is not found:
				create and store new account with a balance of zero
			apply transaction from value to account
			create response with appropriate transaction result, client id and client seq
			forward response to client handling module
		increment adu by 1
		increment decided slot for proposer
		if has previously buffered value for (adu+1):
			handleDecidedValue(value from slot adu+1) //recursive


	- Apply bank transactions in correct order as they are decided.
	- Generate responses to client module

	- A node should generate a response after applying a transaction to an account. The ClientID, ClientSeq and AccountNum fields should be populated from the
	corresponding decided value. A response should be forwarded to the client handling part of your application. It should there be sent to the appropriate client
	if it is connected. Client handling details are described in the next subsection.

*/

// Runs MergeSort algorithm on a slice single
func MergeSort(slice []bank.Account) []bank.Account {

	if len(slice) < 2 {
		return slice
	}
	mid := (len(slice)) / 2
	return Merge(MergeSort(slice[:mid]), MergeSort(slice[mid:]))
}

// Merges left and right slice into newly created slice
func Merge(left, right []bank.Account) []bank.Account {

	size, i, j := len(left)+len(right), 0, 0
	slice := make([]bank.Account, size, size)

	for k := 0; k < size; k++ {
		if i > len(left)-1 && j <= len(right)-1 {
			slice[k] = right[j]
			j++
		} else if j > len(right)-1 && i <= len(left)-1 {
			slice[k] = left[i]
			i++
		} else if left[i].Number < right[j].Number {
			slice[k] = left[i]
			i++
		} else {
			slice[k] = right[j]
			j++
		}
	}
	return slice
}
