package main

import (
	"github.com/gonutz/ide/w32"
)

// Variables to be set via -ldflags -X for Windows standalone binary.
var (
	ffmpegX  string
	tmpX     string
	logfileX string
)

func init() {
	// hideConsole()
	if ffmpegX != "" {
		ffmpeg = ffmpegX
	}
	if tmpX != "" {
		tmp = tmpX
	}
	if logfileX != "" {
		logfile = logfileX
	}
	verbose = true
}

// hideConsole hides the console if -H windowsgui is not specified.
// Since with "-H windowsgui" all subprocesses launched also receive windows.
// This results in several blank console windows open when executing ffmepg
// commands.
//
// Note: there is a delay, so you will see the console window briefly on startup
// before it disappears.
func hideConsole() {
	console := w32.GetConsoleWindow()
	if console == 0 {
		return // no console attached
	}
	// If this application is the process that created the console window, then
	// this program was not compiled with the -H=windowsgui flag and on start-up
	// it created a console along with the main application window. In this case
	// hide the console window.
	// See
	// http://stackoverflow.com/questions/9009333/how-to-check-if-the-program-is-run-from-a-console
	_, consoleProcID := w32.GetWindowThreadProcessId(console)
	if w32.GetCurrentProcessId() == consoleProcID {
		w32.ShowWindowAsync(console, w32.SW_HIDE)
	}
}
