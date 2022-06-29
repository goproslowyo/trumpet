package util

import (
	"fmt"
	"io/ioutil"
	"log"
	"math/rand"
	"os/exec"
	"strings"
	"time"
)

// Not using "command -v" because it doesn't work with Windows.
// testArg will usually be something like --version.
func CheckInstalled(program, testArg string) bool {
	cmd := exec.Command(program, testArg)
	if err := cmd.Run(); err != nil {
		return false
	}
	return true
}

// Loop through announcements dir to create array of greetings.
func GetHeraldSound(announcement_dir string) string {
	var announcement []string
	files, err := ioutil.ReadDir(announcement_dir)

	if err != nil {
		log.Fatal(err)
	}

	for _, file := range files {
		if strings.HasSuffix(file.Name(), ".opus") {
			announcement = append(announcement, file.Name())
		}
	}

	rand.Seed(time.Now().UnixNano())
	return fmt.Sprintf("%s\n", announcement[rand.Intn(len(announcement))])
}
