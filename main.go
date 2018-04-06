package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"os"
	"strconv"
	"strings"

	"github.com/araddon/gou"
	"github.com/google/go-github/github"
	"golang.org/x/oauth2"
)

func main() {
	gou.SetLogger(log.New(os.Stderr, "", 0), "debug")
	gou.SetColorOutput()

	p := &ghp{ctx: context.Background()}

	flag.StringVar(&p.orgrepo, "orgrepo", "", "Org/Repo where project currently resides")
	flag.StringVar(&p.org, "org", "", "Org where project should reside")
	flag.IntVar(&p.projectNumber, "project-number", -1, "Project Number from url (not id, project number)")
	flag.BoolVar(&p.deletePrjectFirst, "delete-project-if-exists", false, "Should we delete the same-name project in org if exists?")
	flag.Parse()

	ctx := context.Background()
	ts := oauth2.StaticTokenSource(
		&oauth2.Token{AccessToken: os.Getenv("GITHUB_TOKEN")},
	)
	tc := oauth2.NewClient(ctx, ts)

	p.client = github.NewClient(tc)

	p.Run()
}

type ghp struct {
	ctx               context.Context
	client            *github.Client
	pnew              *github.Project
	repo              string
	orgrepo           string
	org               string
	projectNumber     int
	deletePrjectFirst bool
}

func (m *ghp) Run() {
	//m.listIssues()
	m.findProjectAndCopy()
}
func (m *ghp) findProjectAndCopy() {
	parts := strings.Split(m.orgrepo, "/")
	if len(parts) != 2 {
		gou.Errorf("exptected orgrepo like   org/repo but got %q", m.orgrepo)
		os.Exit(1)
	}
	m.repo = parts[1]
	gou.Debugf("getting org=%q repo=%q project number %v", parts[0], m.repo, m.projectNumber)

	items, _, err := m.client.Repositories.ListProjects(context.Background(), parts[0], m.repo, nil)
	dieIfErr("could not list projects ", err)
	for _, item := range items {
		//gou.Debugf("id=%-9d %v", item.GetID(), item.GetName())
		if item.GetNumber() == m.projectNumber {
			m.handleProject(item)
		}
	}
}

func (m *ghp) deleteProjectIfExists(p *github.Project) {
	items, _, err := m.client.Organizations.ListProjects(context.Background(), m.org, nil)
	dieIfErr("list projects", err)
	for _, item := range items {
		if p.GetName() == item.GetName() {
			gou.Warnf("We are about to delete %q in org %q please type Y to verify", item.GetName(), m.org)
			if askForConfirmation() {
				m.client.Projects.DeleteProject(m.ctx, item.GetID())
			}
		}
	}
}

func (m *ghp) createProject(p *github.Project) {

	//m.client.Projects.DeleteProject(m.ctx, int64(1414764))
	data := &github.ProjectOptions{
		Name: p.GetName(),
		Body: p.GetBody(),
	}
	pnew, _, err := m.client.Organizations.CreateProject(m.ctx, m.org, data)
	if err != nil {
		gou.Errorf("error %v", err)
	}
	m.pnew = pnew
}

func (m *ghp) moveColumn(col *github.ProjectColumn) *github.ProjectColumn {
	data := &github.ProjectColumnOptions{
		Name: col.GetName(),
	}
	newCol, _, err := m.client.Projects.CreateProjectColumn(m.ctx, m.pnew.GetID(), data)
	if err != nil {
		gou.Errorf("error %v", err)
	}
	return newCol
}
func (m *ghp) moveCard(newCol *github.ProjectColumn, card *github.ProjectCard) {
	data := &github.ProjectCardOptions{
		Note: card.GetNote(),
	}
	if data.Note == "" {
		parts := strings.Split(card.GetContentURL(), "/")
		iv, err := strconv.ParseInt(parts[len(parts)-1], 10, 64)
		//gou.Debugf("%v %v", iv, err)
		dieIfErr("crap", err)

		//owner string, repo string, number int
		issue, _, err := m.client.Issues.Get(context.Background(), m.org, m.repo, int(iv))
		dieIfErr("could not get issue", err)
		data.ContentID = issue.GetID()
		data.ContentType = "Issue"
	}

	// by, _ := json.MarshalIndent(card, "", "  ")
	// gou.Debugf("%s", string(by))
	// gou.Debugf("creating card %+v", data)
	_, _, err := m.client.Projects.CreateProjectCard(m.ctx, newCol.GetID(), data)
	if err != nil {
		gou.Errorf("error %v", err)
	}
}
func (m *ghp) handleProject(p *github.Project) {

	if m.deletePrjectFirst {
		m.deleteProjectIfExists(p)
	}

	m.createProject(p)

	by, _ := json.MarshalIndent(p, "", "  ")
	gou.Debugf("%s", string(by))
	items, _, err := m.client.Projects.ListProjectColumns(context.Background(), p.GetID(), nil)
	dieIfErr("could not get project columns", err)
	for _, col := range items {
		gou.Debugf("Column:  %-9d %v", col.GetID(), col.GetName())
		newCol := m.moveColumn(col)
		cards, _, err := m.client.Projects.ListProjectCards(context.Background(), col.GetID(), nil)
		dieIfErr("could not get project columns", err)
		for _, card := range cards {
			gou.Debugf("Card:  %-9d %v  %v", card.GetID(), card.GetNote(), card.GetColumnURL())
			m.moveCard(newCol, card)
			//m.moveColumn(card)
			// CreateProjectCard(ctx context.Context, columnID int64, opt *ProjectCardOptions) (*ProjectCard, *Response, error)
		}
	}
}
func (m *ghp) listIssues() {
	items, _, err := m.client.Activity.ListIssueEventsForRepository(context.Background(), "lytics", "lio", nil)
	dieIfErr("list issues", err)
	for _, item := range items {
		by, _ := json.MarshalIndent(item, "", "  ")
		gou.Debugf("%s", string(by))
		return
	}
}
func (m *ghp) listProjects() {
	items, _, err := m.client.Organizations.ListProjects(context.Background(), m.org, nil)
	dieIfErr("list projects", err)
	for _, item := range items {
		gou.Debugf("%-9d  %s", item.GetID(), item.GetName())
	}
}
func dieIfErr(msg string, err error) {
	if err != nil {
		//gou.Errorf("could not read %v", err)
		gou.LogD(4, gou.ERROR, msg, err)
		os.Exit(1)
	}
}

// https://gist.github.com/albrow/5882501
func askForConfirmation() bool {
	var response string
	_, err := fmt.Scanln(&response)
	if err != nil {
		log.Fatal(err)
	}
	okayResponses := []string{"y", "Y", "yes", "Yes", "YES"}
	nokayResponses := []string{"n", "N", "no", "No", "NO"}
	if containsString(okayResponses, response) {
		return true
	} else if containsString(nokayResponses, response) {
		return false
	} else {
		fmt.Println("Please type yes or no and then press enter:")
		return askForConfirmation()
	}
}

// posString returns the first index of element in slice.
// If slice does not contain element, returns -1.
func posString(slice []string, element string) int {
	for index, elem := range slice {
		if elem == element {
			return index
		}
	}
	return -1
}

// containsString returns true iff slice contains element
func containsString(slice []string, element string) bool {
	return !(posString(slice, element) == -1)
}
