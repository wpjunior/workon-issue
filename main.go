package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path"
	"strconv"
	"strings"

	"github.com/0xAX/notificator"
	"github.com/howeyc/fsnotify"
	"github.com/nightlyone/lockfile"
	"github.com/spf13/viper"
)

type Issue struct {
	ID          int    `json:"id"`
	IID         int    `json:"iid"`
	Description string `json:"description"`
}

var notify *notificator.Notificator

func init() {
	viper.SetConfigType("yaml")
	viper.SetConfigName("config")
	viper.AddConfigPath("$HOME/.config/workon-issue")
	err := viper.ReadInConfig()
	if err != nil {
		log.Fatal(err)
	}

	notify = notificator.New(notificator.Options{
		AppName: "workon-issue",
	})
}

func main() {
	if len(os.Args) < 2 {
		fmt.Printf("USAGE: %s [ISSUE]\n", os.Args[0])
		os.Exit(1)
	}
	issueID, err := strconv.Atoi(os.Args[1])
	if err != nil {
		fmt.Printf("Bad number: %s\n", os.Args[1])
		os.Exit(1)
	}

	err = lockIssue(issueID)
	if err != nil {
		log.Fatal(err)
	}

	url := viper.GetString("gitlab.url")
	repo := viper.GetString("gitlab.repo")
	token := viper.GetString("gitlab.token")
	editor := viper.GetString("editor")

	baseURL := fmt.Sprintf(
		"%s/api/v3/projects/%s",
		url,
		strings.Replace(repo, "/", "%2F", 1),
	)

	issue, err := getIssue(baseURL, token, issueID)

	if err != nil {
		log.Fatal(err)
	}

	dir := path.Join(os.Getenv("HOME"), ".config", "workon-issue", "issues")
	err = os.MkdirAll(dir, 0700)
	if err != nil {
		log.Fatal(err)
	}

	filePath := path.Join(dir, fmt.Sprintf("%d.org", issueID))
	fillFile(filePath, issue)
	go openEditor(editor, filePath)
	go watcher(baseURL, token, filePath, issue)
	select {}

}

func fillFile(filePath string, issue *Issue) error {
	err := ioutil.WriteFile(filePath, []byte(issue.Description), 0600)
	if err != nil {
		return err
	}
	return nil
}

func openEditor(editor, filePath string) {
	args := strings.Split(editor, " ")
	args = append(args, filePath)
	err := exec.Command(args[0], args[1:]...).Run()
	if err != nil {
		fmt.Printf("Failed to open editor: %s\n", err.Error())
	}
}

func watcher(baseURL, token, filePath string, issue *Issue) {
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		log.Fatal(err)
	}

	go func() {
		for {
			select {
			case ev := <-watcher.Event:
				if !ev.IsModify() {
					continue
				}
				updateIssue(baseURL, token, filePath, issue)
			case err := <-watcher.Error:
				log.Fatal(err)
			}
		}
	}()

	err = watcher.Watch(filePath)
	if err != nil {
		log.Fatal(err)
	}
}

func getIssue(baseURL, token string, issue int) (*Issue, error) {
	req, err := http.NewRequest(http.MethodGet, fmt.Sprintf("%s/issues?iid=%d", baseURL, issue), nil)
	if err != nil {
	}
	req.Header.Set("PRIVATE-TOKEN", token)

	resp, err := http.DefaultClient.Do(req)

	if err != nil {
		return nil, err
	}

	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("HTTP Error: %d", resp.StatusCode)
	}

	var list []*Issue

	err = json.NewDecoder(resp.Body).Decode(&list)

	if err != nil {
		return nil, err
	}

	if len(list) == 0 {
		return nil, errors.New("Issue not found")
	}

	return list[0], nil
}

func updateIssue(baseURL, token, filePath string, issue *Issue) {
	fmt.Println("Updating issue")
	newDescription, err := ioutil.ReadFile(filePath)
	if err != nil {
		notifyError(err)
		return
	}
	values := url.Values{}
	values.Set("description", string(newDescription))
	req, err := http.NewRequest(
		http.MethodPut,
		fmt.Sprintf("%s/issues/%d?%s", baseURL, issue.ID, values.Encode()),
		nil,
	)
	if err != nil {
		notifyError(err)
		return
	}
	req.Header.Set("PRIVATE-TOKEN", token)
	resp, err := http.DefaultClient.Do(req)

	if err != nil {
		notifyError(err)
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusOK {
		notify.Push(
			"Updated issue",
			fmt.Sprintf("Issue %d was updated sucessfully", issue.IID),
			"",
			notificator.UR_CRITICAL,
		)
	} else {
		notifyError(fmt.Errorf("HTTP Error: %d", resp.StatusCode))
	}
}

func notifyError(err error) {
	notify.Push("Failed to update issue", err.Error(), "", notificator.UR_CRITICAL)
}

func lockIssue(issueID int) error {
	dir := path.Join(os.Getenv("HOME"), ".config", "workon-issue", "locks")
	err := os.MkdirAll(dir, 0700)
	if err != nil {
		return err
	}
	filePath := path.Join(dir, fmt.Sprintf("%d.lock", issueID))

	lock, err := lockfile.New(filePath)
	if err != nil {
		return err
	}
	return lock.TryLock()
}
