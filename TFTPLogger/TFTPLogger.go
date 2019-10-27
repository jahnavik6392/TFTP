package TFTPLogger

import (
	"io"
	"log"
	"os"
)

var logger *log.Logger
var reqLogger *log.Logger
var logfile *os.File
var requestLogfile *os.File

//var logWriter io.Writer

//Function to initialize application logger and request logger
func Initialize() {
	//creating log file to save application logs
	var err error
	logfile, err = os.Create("./logs/log")
	if err != nil {
		panic(err)
	}

	//creating log file to save request logs
	var err1 error
	requestLogfile, err1 = os.Create("./logs/requestLog")
	if err1 != nil {
		panic(err1)
	}

	logWriter := io.Writer(logfile)
	logger = log.New(logWriter, "", log.LstdFlags)

	requestWriter := io.Writer(requestLogfile)
	reqLogger = log.New(requestWriter, "", log.LstdFlags)
}

//specifying which log file we want to write the log to
func InitializeWithRequestAndApplicationLogger(requestLogWriter *io.Writer, appLogWriter *io.Writer) {
	reqLogger = log.New(*requestLogWriter, "", log.LstdFlags)
	logger = log.New(*appLogWriter, "", log.LstdFlags)
}

//Function to get the application logger
func GetApplicationLogger() *log.Logger {
	return logger
}

func Error(format string, args ...interface{}) {
	logger.Printf("[ERROR]"+format, args...)
}

//Function to get the request logger
func GetRequestLogger() *log.Logger {
	return reqLogger
}

func DestroyLogger() {
	logfile.Close()
	requestLogfile.Close()
}
