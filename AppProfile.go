package gocore

import (
	"os"
	"runtime"
	"runtime/pprof"
)

func StartCPUProfile() {
	MakeSureDirExists("profile")
	f, err := os.Create("profile/cpu.prof")
	if err != nil {
		Log().Fatal().Err(err).Msg("Could not create CPU Profile")
		return
	}
	if err := pprof.StartCPUProfile(f); err != nil {
		Log().Fatal().Err(err).Msg("Could not start CPU Profile")
		return
	}
}
func StopCPUProfile() {
	pprof.StopCPUProfile()
}

func CaptureMemoryHeap(subfix string) {
	MakeSureDirExists("profile")
	f, err := os.Create("profile/mem_" + subfix + ".heap")
	if err != nil {
		Log().Fatal().Err(err).Msg("Could not create Memory Profile")
		f.Close()
		return
	}
	runtime.GC() // get up-to-date statistics
	if err := pprof.WriteHeapProfile(f); err != nil {
		Log().Fatal().Err(err).Msg("Could not write Memory Profile")
		f.Close()
		return
	}
	f.Close()
}