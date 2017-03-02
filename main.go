package main

import (
	"archive/zip"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"os/exec"
	"syscall"

	"github.com/codeskyblue/httpfs"
	"github.com/wmbest2/android/apk"
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
	"parse": cmdParse,
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

func main() {
	if len(os.Args) > 1 {
		subCmd := os.Args[1]
		args := os.Args[2:]
		fn, ok := cmds[subCmd]
		if ok {
			fn(args...)
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
