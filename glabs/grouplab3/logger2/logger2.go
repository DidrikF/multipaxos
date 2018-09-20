package logger2

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"
)

type Logger struct {
	LogDir     string
	SystemLog  *os.File
	NetworkLog *os.File
	Severity   int
	Loud       bool
	NodeId     int
	NetOnly    bool
	Save       bool
	logChan    chan LogMessage
	stopChan   chan struct{}
	WG         *sync.WaitGroup
}

type LogMessage struct {
	D string
	S string
	M string
}

var (
	LogSeverities = map[string]int{
		"emerg":   0,
		"alert":   1,
		"crit":    2,
		"err":     3,
		"warning": 4,
		"notice":  5,
		"info":    6,
		"debug":   7,
	}
	readOnlyMutex sync.RWMutex
)

func CreateAndStartLogger(nodeId int, logDir string, systemLog string, networkLog string, severity string, loud bool, save bool, netonly bool, wg *sync.WaitGroup) (*Logger, error) {
	logger := &Logger{
		Loud:     loud,
		NodeId:   nodeId,
		NetOnly:  netonly,
		Save:     save,
		logChan:  make(chan LogMessage, 32),
		stopChan: make(chan struct{}),
		WG:       wg,
	}

	if severityNumber, ok := LogSeverities[severity]; ok == false {
		return logger, errors.New("Logger: Unknown severity level \"" + severity + "\" given")
	} else {
		logger.Severity = severityNumber
	}

	//check log directory exists, if not create it
	//os.MkdirAll(folderPath, os.ModePerm); //If path is already a directory, MkdirAll does nothing and returns nil.
	if logger.Save == true {
		err := logger.SetLogDir(logDir)
		if err != nil {
			return logger, err
		}

		err = logger.SetSystemLog(systemLog)
		if err != nil {
			logger.SystemLog.Close()
			return logger, err
		}

		err = logger.SetNetworkLog(networkLog)
		if err != nil {
			logger.SystemLog.Close()
			logger.NetworkLog.Close()
			return logger, err
		}

		logger.WG.Add(1)
		go func() {
			for {
				select {
				case logMessage := <-logger.logChan:
					err := logger.write(logMessage)
					if err != nil {
						fmt.Printf(err.Error())
					}
				case <-logger.stopChan:
					for logMessage := range logger.logChan { //logs empty messages
						err := logger.write(logMessage)
						if err != nil {
							fmt.Printf(err.Error())
						}
					}
					logger.NetworkLog.Close()
					logger.SystemLog.Close()
					logger.WG.Done()
					return
				}
			}
		}()
	}

	return logger, nil
}

func (log *Logger) Log(msg LogMessage) { //make safe for concurrent calls
	readOnlyMutex.RLock()
	defer readOnlyMutex.RUnlock()
	if _, ok := LogSeverities[msg.S]; ok == false {
		fmt.Println("Logger: Unknown severity level \"" + msg.S + "\" given")
		return
	}
	if LogSeverities[msg.S] > log.Severity { //only log that which has lower or equal severity level to that of the logger.
		return
	}

	timeStamp := time.Now().Format(time.RFC3339)
	logMessage := timeStamp + "  -  " + strings.ToUpper(msg.S) + "  -  " + msg.M

	if msg.D == "system" {
		if log.Loud == true && log.NetOnly == false {
			fmt.Println(logMessage)
		}
		if log.Save == true {
			log.logChan <- msg
		}
	} else if msg.D == "network" {
		if log.Loud == true {
			fmt.Println(logMessage)
		}
		if log.Save == true {
			log.logChan <- msg
		}
	}
	return
}

//Log string to Logger.destination
func (log *Logger) write(msg LogMessage) error {
	//check if its a zero LogMessage
	if msg.M == "" {
		return nil
	}

	timeStamp := time.Now().Format(time.RFC3339)
	logMessage := timeStamp + "  -  " + strings.ToUpper(msg.S) + "  -  " + msg.M

	if msg.D == "system" {
		_, err := log.SystemLog.WriteString(logMessage + "\n")
		if err != nil {
			return errors.New("Logger: Failed to write to system log file, with error: " + err.Error())
		}
	} else if msg.D == "network" {
		_, err := log.NetworkLog.WriteString(logMessage + "\n")
		if err != nil {
			return errors.New("Logger: Failed to write to network log file, with error: " + err.Error())
		}
	}
	return nil
}

func (log *Logger) SetLogDir(name string) error {
	if _, err := os.Stat(name); os.IsNotExist(err) {
		err := os.Mkdir(name, os.ModePerm)
		if err != nil {
			return errors.New("Logger: Could not create log directory with path: " + name)
		}
	}
	log.LogDir = name
	return nil
}

func (log *Logger) SetSystemLog(name string) error {
	nodeName := name + "_" + strconv.Itoa(log.NodeId) + ".txt"
	fileName := filepath.Join(log.LogDir, nodeName)
	fmt.Println(fileName)
	file, err := os.OpenFile(fileName, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return errors.New("Logger: Failed to set system log file to: " + fileName)
	}
	log.SystemLog = file
	return nil
}

func (log *Logger) SetNetworkLog(name string) error {
	nodeName := name + "_" + strconv.Itoa(log.NodeId) + ".txt"
	fileName := filepath.Join(log.LogDir, nodeName)
	file, err := os.OpenFile(fileName, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return errors.New("Logger: Failed to set system log file to: " + fileName)
	}
	log.NetworkLog = file
	return nil
}

func (log *Logger) Stop() {
	close(log.logChan)
	log.stopChan <- struct{}{}
}
