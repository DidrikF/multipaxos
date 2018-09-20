package logger

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
	SystemLog  string
	NetworkLog string
	Severity   string
	Loud       bool
	NodeId     int
	NetOnly    bool
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
	sysLogMutex = &sync.Mutex{}
	netLogMutex = &sync.Mutex{}
)

func CreateLogger(logDir string, systemLog string, networkLog string, severity string, loud bool, nodeId int, netonly bool) (Logger, error) {
	logger := Logger{}
	logger.Severity = severity
	logger.Loud = loud
	logger.NodeId = nodeId
	logger.NetOnly = netonly
	//check log directory is exists, if not create it (relative to grouplab1 folder)
	//os.MkdirAll(folderPath, os.ModePerm); //If path is already a directory, MkdirAll does nothing and returns nil.
	err := logger.SetLogDir(logDir)
	if err != nil {
		return Logger{}, err
	}

	err = logger.SetSystemLog(systemLog)
	if err != nil {
		return Logger{}, err
	}

	err = logger.SetNetworkLog(networkLog)
	if err != nil {
		return Logger{}, err
	}

	if _, ok := LogSeverities[severity]; ok == false {
		return Logger{}, errors.New("Logger: Unknown severity level \"" + severity + "\" given")
	}

	logger.LogDir = logDir

	return logger, nil
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
	file.Close()
	if err != nil {
		return errors.New("Logger: Failed to set system log file to: " + fileName)
	}
	log.SystemLog = nodeName
	return nil
}

func (log *Logger) SetNetworkLog(name string) error {
	nodeName := name + "_" + strconv.Itoa(log.NodeId) + ".txt"
	fileName := filepath.Join(log.LogDir, nodeName)
	file, err := os.OpenFile(fileName, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	file.Close()
	if err != nil {
		return errors.New("Logger: Failed to set system log file to: " + fileName)
	}
	log.NetworkLog = nodeName
	return nil
}

//Log string to Logger.destination
func (log *Logger) Log(dest string, severity string, msg string) error {
	if _, ok := LogSeverities[severity]; ok == false {
		return errors.New("Logger: Unknown severity level \"" + severity + "\" given")
	}
	if LogSeverities[severity] > LogSeverities[log.Severity] { //only log that which has lower or equal severity level to that of the logger.
		return nil
	}

	if dest == "system" {
		return log.logToSystemLog(severity, msg)
	} else if dest == "network" {
		return log.logToNetworkLog(severity, msg)
	}
	return nil
}

func (log *Logger) logToSystemLog(severity string, msg string) error {
	//Create message:
	timeStamp := time.Now().Format(time.RFC3339) //.String() //time.Now().Format(time.RFC3339)
	logMessage := timeStamp + "  -  " + strings.ToUpper(severity) + "  -  " + msg
	if log.Loud == true && log.NetOnly == false {
		fmt.Println(logMessage)
	}

	fileName := filepath.Join(log.LogDir, log.SystemLog)
	sysLogMutex.Lock()
	file, err := os.OpenFile(fileName, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return errors.New("Logger: Failed to open system log file, with error: " + err.Error())
	}

	_, err = file.WriteString(logMessage + "\n")
	file.Close()
	sysLogMutex.Unlock()
	if err != nil {
		return errors.New("Logger: Failed to write to system log file, with error: " + err.Error())
	}
	return nil
}

func (log *Logger) logToNetworkLog(severity string, msg string) error { //Example log format: 127.0.0.1 user-identifier frank [10/Oct/2000:13:55:36 -0700] "GET /apache_pb.gif HTTP/1.0" 200 2326
	//Create message:
	timeStamp := time.Now().Format(time.RFC3339) //.String() //time.Now().Format(time.RFC3339)
	logMessage := timeStamp + "  -  " + strings.ToUpper(severity) + "  -  " + msg
	if log.Loud == true {
		fmt.Println(logMessage)
	}

	fileName := filepath.Join(log.LogDir, log.NetworkLog)
	netLogMutex.Lock()
	file, err := os.OpenFile(fileName, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return errors.New("Logger: Failed to open network log file, with error: " + err.Error())
	}
	_, err = file.WriteString(logMessage + "\n")
	file.Close()
	netLogMutex.Unlock()
	if err != nil {
		return errors.New("Logger: Failed to write to network log file, with error: " + err.Error())
	}
	return nil
}

/*
//Fatal logs string to Logger.destination and exit program with error code 1
func (log *Logger) Fatal(dest string, severity string, msg string) {
	log.Log(dest, severity, msg)
	fmt.Println("Fatal() is not implemented property.")
	//os.Exit(1)
	return
}


//ReadLog returns the contents of the given file
func (log *Logger) ReadLog(fileName string) ([]string, error) {
	//read log into slice of strings
	return []string{}, nil
}
*/
