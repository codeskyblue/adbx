package main

import (
	"archive/zip"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"github.com/cheggaaa/pb"
	"github.com/codeskyblue/httpfs"
	"github.com/franela/goreq"
	"github.com/wmbest2/android/apk"
	goadb "github.com/yosemite-open/go-adb"
)

func ErrToExitCodo(err error) int {
	if err == nil {
		return 0
	}
	if exiterr, ok := err.(*exec.ExitError); ok {
		// The program has exited with an exit code != 0

		// This works on both Unix and Windows. Although package
		// syscall is generally platform dependent, WaitStatus is
		// defined for both Unix and Windows and in both cases has
		// an ExitStatus() method with the same signature.
		if status, ok := exiterr.Sys().(syscall.WaitStatus); ok {
			return status.ExitStatus()
		}
	}
	return 127
}

var cmds = map[string]func(...string){
	"parse":   cmdParse,
	"install": cmdInstall,
}

func readManifestFromZip(zrd *zip.Reader) (data []byte, err error) {
	for _, f := range zrd.File {
		if f.Name != "AndroidManifest.xml" {
			continue
		}
		rc, er := f.Open()
		if er != nil {
			return nil, er
		}
		data, err = ioutil.ReadAll(rc)
		rc.Close()
		return
	}
	return nil, fmt.Errorf("File not found: AndroidManifest.xml")
}

func parseManifest(data []byte) {
	var manifest apk.Manifest
	err := apk.Unmarshal(data, &manifest)
	if err != nil {
		log.Fatal(err)
	}
	var launchActivity apk.AppActivity
	for _, act := range manifest.App.Activity {
		for _, intent := range act.IntentFilter {
			if intent.Action.Name == "android.intent.action.MAIN" &&
				intent.Category.Name == "android.intent.category.LAUNCHER" {
				launchActivity = act
				goto FOUND
			}
		}
	}
FOUND:
	output, _ := json.MarshalIndent(map[string]string{
		"packageName":    manifest.Package,
		"launchActivity": launchActivity.Name,
	}, "", "    ")
	fmt.Println(string(output))
	// fmt.Println(manifest.Package)
	// fmt.Println(launchActivity.Name)
	// fmt.Printf("adb shell am start -n %s/%s\n", manifest.Package, launchActivity.Name)
	//out, _ := xml.MarshalIndent(manifest, "", "\t")
	//fmt.Printf("%s\n", string(out))
}

func requireAtleastArgs(n int, args []string) {
	if len(args) != n {
		log.Fatalf("require at least %d args", n)
	}
}

func cmdParse(args ...string) {
	requireAtleastArgs(1, args)

	url := args[0]
	file, err := httpfs.Open(url)
	if err != nil {
		log.Fatal(err)
	}
	zrd, err := zip.NewReader(file, file.Size())
	if err != nil {
		log.Fatal(err)
	}
	data, err := readManifestFromZip(zrd)
	if err != nil {
		log.Fatal(err)
	}
	parseManifest(data)
}

func cmdInstall(args ...string) {
	filename := args[len(args)-1]
	args = args[:len(args)-1]

	name := filepath.Base(filename)
	if !strings.HasSuffix(name, ".apk") {
		name += ".apk"
	}
	dest := "/data/local/tmp/" + name
	var reader io.Reader
	var length int64
	var err error
	if strings.HasPrefix(filename, "http://") || strings.HasPrefix(filename, "https://") {
		log.Println("Pushing HTTP file to device")
		res, err := goreq.Request{Uri: filename}.Do()
		if err != nil {
			log.Fatal(err)
		}
		fmt.Sscanf(res.Header.Get("Content-Length"), "%d", &length)
		defer res.Body.Close()
		reader = res.Body
	} else {
		file, err := os.Open(filename)
		if err != nil {
			log.Fatal(err)
		}
		defer file.Close()
		reader = file
	}
	bar := pb.New(int(length)).SetUnits(pb.U_BYTES) //.SetRefreshRate(time.Millisecond * 100)
	bar.ShowSpeed = true
	bar.ShowTimeLeft = true
	bar.ManualUpdate = true

	done := make(chan bool)
	//if length == 0 {
	//	log.Println("Source size is unknown, can not show progress")
	//} else {
	bar.Start()
	go func() {
		for {
			fstat, er := device.Stat(dest)
			if er == nil {
				bar.Set(int(fstat.Size))
				bar.Update()
			}
			select {
			case <-time.After(time.Millisecond * 200):
			case <-done:
				return
			}
		}
	}()
	_, err = device.WriteToFile(dest, reader, 0644)
	if err != nil {
		log.Fatal(err)
	}

	// Success push file
	done <- true
	bar.Finish()

	log.Printf("Install apk to android system (%s)", dest)
	defer func() {
		// log.Println("Clean up apk file")
		device.RunCommand("rm", dest)
	}()
	output, err := device.RunCommand("pm", append([]string{"install"}, append(args, dest)...)...)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Print(output)
}

var device *goadb.Device

func main() {
	if len(os.Args) > 1 {
		args := os.Args[1:]
		adb, err := goadb.New()
		if err != nil {
			log.Fatal(err)
		}
		if args[0] == "-s" {
			device = adb.Device(goadb.DeviceWithSerial(args[1]))
			args = args[2:]
		} else {
			device = adb.Device(goadb.AnyDevice())
		}
		subCmd := args[0]
		fn, ok := cmds[subCmd]
		if ok {
			fn(args[1:]...)
			return
		}
	}

	cmd := exec.Command("adb", os.Args[1:]...)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Start(); err != nil {
		if err := cmd.Start(); err != nil {
			log.Fatal(err)
		}
	}
	os.Exit(ErrToExitCodo(cmd.Wait()))
}
