package bankmanager4

import (
	"errors"
	"fmt"
	"sync"

	"github.com/uis-dat520-s18/glabs/grouplab4/bank4"
	"github.com/uis-dat520-s18/glabs/grouplab4/logger4"
	"github.com/uis-dat520-s18/glabs/grouplab4/multipaxos4"
)

type Bankmanager struct {
	l                  *logger4.Logger
	adu                int
	decidedValueBuffer []multipaxos4.DecidedValue
	bankAccounts       map[int]bank4.Account
	responseOut        chan<- multipaxos4.Response
	proposer           *multipaxos4.Proposer

	DecidedValueHistory      []multipaxos4.Value
	TransactionResultHistory []bank4.TransactionResult

	Users []bank4.User

	mutex sync.RWMutex
}

func NewbankManager(responseOut chan<- multipaxos4.Response, prop *multipaxos4.Proposer, logger *logger4.Logger) *Bankmanager {
	bankmanager := &Bankmanager{
		l:                        logger,
		adu:                      -1, //Keep track of the id of highest decided slot.
		decidedValueBuffer:       []multipaxos4.DecidedValue{},
		bankAccounts:             map[int]bank4.Account{},
		responseOut:              responseOut,
		proposer:                 prop,
		DecidedValueHistory:      []multipaxos4.Value{},
		TransactionResultHistory: []bank4.TransactionResult{},

		Users: []bank4.User{},

		mutex: sync.RWMutex{},
	}
	return bankmanager
}

func (bm *Bankmanager) HandleDecidedValue(decidedValue multipaxos4.DecidedValue) {
	// fmt.Printf("#bank4 MANAGER: decidedValue: %v, adu: %v, decidedValueBuffer: %v\n", decidedValue, bm.adu, bm.decidedValueBuffer)
	bm.l.Log(logger4.LogMessage{O: "bankmanager", D: "system", S: "debug", M: fmt.Sprintf("HandleDecidedValue invoked with decidedValue: %+v", decidedValue)})

	bm.proposer.IncrementAllDecidedUpToInRestoreMode()

	if int(decidedValue.SlotID) <= bm.adu {
		// fmt.Println("#bank4 MANAGER: returning to avoid applying the same transaction multiple times!")
		bm.l.Log(logger4.LogMessage{O: "bankmanager", D: "system", S: "notice", M: fmt.Sprintf("DecidedValue's SlotID: %v <= bm.adu: %+v", decidedValue.SlotID, bm.adu)})
		return
	}
	// Record decided value history

	bm.mutex.Lock()
	bm.DecidedValueHistory = append(bm.DecidedValueHistory, decidedValue.Value)
	bm.mutex.Unlock()

	// Save decided valeus for future slots in a buffer
	if int(decidedValue.SlotID) > bm.adu+1 {
		bm.l.Log(logger4.LogMessage{O: "bankmanager", D: "system", S: "notice", M: fmt.Sprintf("DecidedValue's SlotID: %v > bm.adu: %+v+1, the decidedvalue was buffered", decidedValue.SlotID, bm.adu)})
		bm.mutex.Lock()
		bm.decidedValueBuffer = append(bm.decidedValueBuffer, decidedValue)
		bm.mutex.Unlock()
		return
	}
	// At this point we know that decidedValue.SlotID == bm.adu+1 (meaning: the nex one)
	if decidedValue.Value.Noop == false {
		bm.l.Log(logger4.LogMessage{O: "bankmanager", D: "system", S: "debug", M: fmt.Sprintf("Decided value is being processed, decidedvalue: %v", decidedValue)})

		response := multipaxos4.Response{
			ClientID:  decidedValue.Value.ClientID,
			ClientSeq: decidedValue.Value.ClientSeq,
		}
		/** Group Lab 4
		*	Add functinality to create new users, new accounts, handle transfers (+ other transactions)
		 */
		// Create User
		if decidedValue.Value.CreateUser == true {
			err := bm.createUser(decidedValue.Value.User)
			if err != nil {
				response.Error = err.Error()
				bm.responseOut <- response
			} else {
				bm.l.Log(logger4.LogMessage{O: "bankmanager", D: "system", S: "debug", M: fmt.Sprintf("Created user:%v", decidedValue.Value.User)})
				response.User = decidedValue.Value.User
				bm.responseOut <- response
			}
		} else if decidedValue.Value.CreateAccount == true {
			err, account := bm.createAccount(decidedValue.Value.Account)
			if err != nil {
				response.Error = err.Error()
				bm.responseOut <- response
			} else {
				bm.l.Log(logger4.LogMessage{O: "bankmanager", D: "system", S: "debug", M: fmt.Sprintf("Created account:%v", account)})
				response.Account = account
				bm.responseOut <- response
			}
		} else {
			accNum := decidedValue.Value.AccountNum
			if _, ok := bm.bankAccounts[accNum]; ok != true {
				bm.l.Log(logger4.LogMessage{O: "bankmanager", D: "system", S: "debug", M: fmt.Sprintf("There is no account with account number %v", accNum)})
				response.Error = fmt.Sprintf("Transaction failed because there is no account with account number %v", accNum)
			} else {
				// Transfer transactions
				if decidedValue.Value.Txn.Op == bank4.Transfer {
					receiverAccNum := decidedValue.Value.Txn.Receiver
					if _, ok := bm.bankAccounts[receiverAccNum]; ok != true {
						bm.l.Log(logger4.LogMessage{O: "bankmanager", D: "system", S: "debug", M: fmt.Sprintf("There is no account to receive funds with account number %v", receiverAccNum)})
						response.Error = fmt.Sprintf("Transaction failed because there is no account to receive funds with account number %v", receiverAccNum)
					} else {
						account := bm.bankAccounts[accNum]
						receiver := bm.bankAccounts[receiverAccNum]
						// Perform trasaction
						txnResToSender, txnResToReceiver, updatedReceiverAccount := account.ProcessTransfer(decidedValue.Value.Txn, receiver)
						// Save updated state of accounts
						bm.bankAccounts[accNum] = account
						bm.bankAccounts[receiverAccNum] = updatedReceiverAccount

						bm.l.Log(logger4.LogMessage{O: "bankmanager", D: "system", S: "debug", M: fmt.Sprintf("Processed transfer transaction with transaction result: %v", txnResToSender)})

						//record history
						bm.TransactionResultHistory = append(bm.TransactionResultHistory, txnResToSender, txnResToReceiver)

						//build and send response(s)
						// - Send transaction result of the receiver if he is connected + only add result of the receiver if the sender is the owner of that account to.
						response.TxnRes = txnResToSender
						bm.responseOut <- response

					}

				} else { // Deposit, withdraw, balance transactions
					account := bm.bankAccounts[accNum]
					transactionResult := account.Process(decidedValue.Value.Txn) //deposit even if a balance...
					bm.mutex.Lock()
					bm.TransactionResultHistory = append(bm.TransactionResultHistory, transactionResult)
					bm.mutex.Unlock()
					bm.bankAccounts[accNum] = account

					bm.l.Log(logger4.LogMessage{O: "bankmanager", D: "system", S: "debug", M: fmt.Sprintf("Processed transaction with transaction result: %v", transactionResult)})

					response.TxnRes = transactionResult

					bm.responseOut <- response
				}
			}

		}
	}
	bm.adu++
	bm.proposer.IncrementAllDecidedUpTo()
	if nextDecidedValue, ok := bm.hasBufferedValueForNextSlot(); ok == true {
		bm.HandleDecidedValue(nextDecidedValue)
	}
	return
}

func (bm *Bankmanager) findTransactionResultFromDecidedValue() {

}

func (bm *Bankmanager) hasBufferedValueForNextSlot() (dv multipaxos4.DecidedValue, ok bool) {
	for index, decidedValue := range bm.decidedValueBuffer {
		if int(decidedValue.SlotID) == bm.adu+1 {
			bm.decidedValueBuffer = append(bm.decidedValueBuffer[:index], bm.decidedValueBuffer[index+1:]...)
			return decidedValue, true
		}
	}
	return dv, false
}

func (bm *Bankmanager) GetStatus() string {
	bm.mutex.RLock()
	defer bm.mutex.RUnlock()
	bankAccounts := ""
	sortbankAccounts := []bank4.Account{}
	for _, row := range bm.bankAccounts {
		sortbankAccounts = append(sortbankAccounts, row)
	}
	sortbankAccounts = MergeSort(sortbankAccounts)
	for _, account := range sortbankAccounts {
		bankAccounts += fmt.Sprintf("%v\n", account)
	}
	return fmt.Sprintf("#bank4 Manager Status: \n - bank4 Accounts: \n%v", bankAccounts)
}

func (bm *Bankmanager) GetHistory() string {
	bm.mutex.RLock()
	defer bm.mutex.RUnlock()
	dvHistory := ""
	trHistory := ""
	for index, decidedVal := range bm.DecidedValueHistory {
		dvHistory += fmt.Sprintf("%v: %v\n", index, decidedVal)
	}
	for index, transactionRes := range bm.TransactionResultHistory {
		trHistory += fmt.Sprintf("%v: %v\n", index, transactionRes)
	}
	return fmt.Sprintf("#bank4 Manager:\n - Decided Value History:\n%v - Transaction Result History:\n%v", dvHistory, trHistory)
}

/**
*	Group Lab 4
 */
func (bm *Bankmanager) GetUsersAccounts(user bank4.User) (usersAccounts []bank4.Account) {
	bm.mutex.RLock()
	defer bm.mutex.RUnlock()
	for _, account := range bm.bankAccounts {
		if account.Owner.Username == user.Username && account.Owner.Password == user.Password {
			usersAccounts = append(usersAccounts, account)
		}
	}

	return usersAccounts
}

func (bm *Bankmanager) GetUsersAccountHistories(user bank4.User) (usersTransactionResults []bank4.TransactionResult) {
	bm.mutex.RLock()
	defer bm.mutex.RUnlock()
	for _, transactionResult := range bm.TransactionResultHistory {
		if authorized := bm.AuthorizeUser(transactionResult.AccountNum, user); authorized == true {
			usersTransactionResults = append(usersTransactionResults, transactionResult)
		}
	}

	return usersTransactionResults
}

func (bm *Bankmanager) GetAccountStatus(accountNumber int) (error, bank4.Account) {
	bm.mutex.RLock()
	defer bm.mutex.RUnlock()
	if _, ok := bm.bankAccounts[accountNumber]; ok != true {
		return fmt.Errorf("No account with account number %v", accountNumber), bank4.Account{}
	}
	return nil, bm.bankAccounts[accountNumber]
}

func (bm *Bankmanager) GetAllAccountStatuses() []bank4.Account {
	bm.mutex.RLock()
	defer bm.mutex.RUnlock()
	bankAccountBuffer := []bank4.Account{}
	for _, account := range bm.bankAccounts {
		bankAccountBuffer = append(bankAccountBuffer, account)
	}
	return MergeSort(bankAccountBuffer)
}

// maybe also return decided values ? may contain some information that would be nice to display? IDK
func (bm *Bankmanager) GetAccountHistory(accountNumber int) (accountHistory []bank4.TransactionResult) { // OBS: get transfers to an account...
	bm.mutex.RLock()
	defer bm.mutex.RUnlock()
	for _, transactionResult := range bm.TransactionResultHistory {
		if transactionResult.AccountNum == accountNumber {
			accountHistory = append(accountHistory, transactionResult)
		}
	}
	return accountHistory
}

func (bm *Bankmanager) GetAllAccountHistories() ([]multipaxos4.Value, []bank4.TransactionResult) {
	return bm.DecidedValueHistory, bm.TransactionResultHistory
}

func (bm *Bankmanager) createUser(user bank4.User) error {
	if len(user.Username) < 4 {
		return fmt.Errorf("Username '%v' is to short, it needs to be at least 4 characters long", user.Username)
	}
	if len(user.Password) < 4 {
		return errors.New("Password was too short, it needs to be at least 4 characters long")
	}
	bm.mutex.Lock()
	defer bm.mutex.Unlock()
	for _, u := range bm.Users {
		if u.Username == user.Username {
			return fmt.Errorf("Username '%v' is allready taken", user.Username)
		}
	}

	bm.Users = append(bm.Users, user)

	return nil
}

func (bm *Bankmanager) AuthenticateUser(user bank4.User) bool {
	bm.mutex.RLock()
	defer bm.mutex.RUnlock()
	for _, u := range bm.Users {
		if u.Username == user.Username && u.Password == user.Password {
			return true
		}
	}
	return false
}

func (bm *Bankmanager) AuthorizeUser(accountNumber int, user bank4.User) bool {
	bm.mutex.RLock()
	defer bm.mutex.RUnlock()
	for _, account := range bm.bankAccounts {
		if account.Number == accountNumber {
			if account.Owner.Username == user.Username && account.Owner.Password == user.Password {
				return true
			}
		}
	}
	return false
}

func (bm *Bankmanager) createAccount(account bank4.Account) (error, bank4.Account) {
	if len(account.Name) < 1 {
		return fmt.Errorf("An account name needs to be spesified"), account
	}

	accountNumber := bm.getFreeAccountNumber()

	// Populate missing fields.
	account.Number = accountNumber
	account.Balance = 0

	bm.mutex.Lock()
	bm.bankAccounts[account.Number] = account // How to ensure non conclicting account numbers
	bm.mutex.Unlock()

	return nil, account
}

func (bm *Bankmanager) getFreeAccountNumber() int {
	bm.mutex.RLock()
	defer bm.mutex.RUnlock()
	highestNumber := 0
	for _, account := range bm.bankAccounts {
		if account.Number > highestNumber {
			highestNumber = account.Number
		}
	}

	return highestNumber + 1
}

func (bm *Bankmanager) IsUsernameTaken(username string) bool {
	bm.mutex.RLock()
	defer bm.mutex.RUnlock()
	for _, user := range bm.Users {
		if user.Username == username {
			return true
		}
	}
	return false
}

// Runs MergeSort algorithm on a slice single
func MergeSort(slice []bank4.Account) []bank4.Account {

	if len(slice) < 2 {
		return slice
	}
	mid := (len(slice)) / 2
	return Merge(MergeSort(slice[:mid]), MergeSort(slice[mid:]))
}

// Merges left and right slice into newly created slice
func Merge(left, right []bank4.Account) []bank4.Account {

	size, i, j := len(left)+len(right), 0, 0
	slice := make([]bank4.Account, size, size)

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
