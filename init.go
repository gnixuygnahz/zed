package zed

var (
	workdir                         = "./"
	logdir                          = ""
	worklogdir                      = "./log"
	showClientData                  = false
	dataInSupervisor  func(*NetMsg) = nil
	dataOutSupervisor func(*NetMsg) = nil
)

func Init(workDir string, logDir string, showClient bool, inSupervisor func(*NetMsg), outSupervisor func(*NetMsg)) {
	ZLog("Init, Working Dir:\"%s\", Log Dir:\"%s\", showClient:\"%v\"", workDir, logDir, showClient)
	workdir = workDir
	worklogdir = logDir
	showClientData = showClient
	dataInSupervisor = inSupervisor
	dataOutSupervisor = outSupervisor
	MakeNewLogDir()

	/*if zlogfile == nil {
		zlogfile = CreateLogFile("ZLog")
		zlogfile.NewFile()
	}*/
	//os.Exit(0)
}
