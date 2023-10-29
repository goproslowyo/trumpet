package util

import (
	"fmt"
	"math/rand"
	"os"
	"os/exec"
	"strings"

	"go.uber.org/zap"
)

var logger *zap.Logger

// Not using "command -v" because it doesn't work with Windows.
// testArg will usually be something like --version.
func CheckInstalled(program, testArg string) (bool, error) {
	logger, _ = zap.NewDevelopment()
	defer logger.Sync()
	cmd := exec.Command(program, testArg)
	if err := cmd.Run(); err != nil {
		return false, err
	}

	return true, nil
}

// Loop through announcements dir to create array of greetings.
func GetHeraldSound(announcement_dir string) string {
	var announcement []string
	files, err := os.ReadDir(announcement_dir)
	if err != nil {
		logger.Fatal(err.Error())
	}

	for _, file := range files {
		if strings.HasSuffix(file.Name(), ".opus") {
			announcement = append(announcement, file.Name())
		}
	}

	seed := rand.Int63()
	rand.New(rand.NewSource(seed))
	afile := announcement[rand.Intn(len(announcement))]
	fmt.Println("Chose announcement file: %s", afile)
	return fmt.Sprintf("%s/%s", announcement_dir, afile)
}
