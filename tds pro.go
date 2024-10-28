package main

import (
	"encoding/csv"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"
)

const (
	githubToken = "_"
	baseURL     = "https://api.github.com"
)

type User struct {
	Login       string `json:"login"`
	Name        string `json:"name"`
	Company     string `json:"company"`
	Location    string `json:"location"`
	Email       string `json:"email"`
	Hireable    bool   `json:"hireable"`
	Bio         string `json:"bio"`
	PublicRepos int    `json:"public_repos"`
	Followers   int    `json:"followers"`
	Following   int    `json:"following"`
	CreatedAt   string `json:"created_at"`
}

type Repo struct {
	Login           string `json:"login"`
	FullName        string `json:"full_name"`
	CreatedAt       string `json:"created_at"`
	StargazersCount int    `json:"stargazers_count"`
	WatchersCount   int    `json:"watchers_count"`
	Language        string `json:"language"`
	HasProjects     bool   `json:"has_projects"`
	HasWiki         bool   `json:"has_wiki"`
	LicenseName     string `json:"license_name"`
}

func fetchUsersInShanghai() ([]User, error) {
	var users []User
	query := "location:Shanghai+followers:>200"
	page := 1
	perPage := 100

	for {
		url := fmt.Sprintf("%s/search/users?q=%s&per_page=%d&page=%d", baseURL, query, perPage, page)
		req, err := http.NewRequest("GET", url, nil)
		if err != nil {
			return nil, err
		}
		req.Header.Set("Authorization", "token "+githubToken)

		client := &http.Client{Timeout: 10 * time.Second}
		resp, err := client.Do(req)
		if err != nil {
			return nil, err
		}
		defer resp.Body.Close()

		var result struct {
			Items []User `json:"items"`
		}
		if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
			return nil, err
		}

		users = append(users, result.Items...)
		if len(result.Items) < perPage {
			break
		}
		page++
	}
	return users, nil
}

func fetchUserDetailsConcurrently(users []User) []User {
	var wg sync.WaitGroup
	ch := make(chan User, len(users))

	for _, user := range users {
		wg.Add(1)
		go func(login string) {
			defer wg.Done()
			userDetail, err := fetchUserDetails(login) // Fixed variable name
			if err == nil {
				ch <- userDetail
			}
		}(user.Login)
	}

	go func() {
		wg.Wait()
		close(ch)
	}()

	var detailedUsers []User
	for user := range ch {
		detailedUsers = append(detailedUsers, user)
	}
	return detailedUsers
}

func fetchUserDetails(username string) (User, error) {
	url := fmt.Sprintf("%s/users/%s", baseURL, username)
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return User{}, err
	}
	req.Header.Set("Authorization", "token "+githubToken)

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return User{}, err
	}
	defer resp.Body.Close()

	var user User
	if err := json.NewDecoder(resp.Body).Decode(&user); err != nil {
		return User{}, err
	}
	user.Company = cleanCompanyName(user.Company)
	return user, nil
}

func cleanCompanyName(company string) string {
	company = strings.TrimSpace(strings.ToUpper(company))
	if strings.HasPrefix(company, "@") {
		return company[1:]
	}
	return company
}

func fetchUserReposConcurrently(users []User) []Repo {
	var wg sync.WaitGroup
	repoCh := make(chan []Repo, len(users))

	for _, user := range users {
		wg.Add(1)
		go func(login string) {
			defer wg.Done()
			repos, err := fetchUserRepos(login)
			if err == nil {
				repoCh <- repos
			}
		}(user.Login)
	}

	go func() {
		wg.Wait()
		close(repoCh)
	}()

	var allRepos []Repo
	for repos := range repoCh {
		allRepos = append(allRepos, repos...)
	}
	return allRepos
}

func fetchUserRepos(username string) ([]Repo, error) {
	var repos []Repo
	url := fmt.Sprintf("%s/users/%s/repos?per_page=500", baseURL, username)
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "token "+githubToken)

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if err := json.NewDecoder(resp.Body).Decode(&repos); err != nil {
		return nil, err
	}

	for i := range repos {
		repos[i].Login = username
	}
	return repos, nil
}

func saveUsersToCSV(users []User) error {
	file, err := os.Create("users.csv")
	if err != nil {
		return err
	}
	defer file.Close()

	writer := csv.NewWriter(file)
	defer writer.Flush()

	writer.Write([]string{"login", "name", "company", "location", "email", "hireable", "bio", "public_repos", "followers", "following", "created_at"})
	for _, user := range users {
		writer.Write([]string{
			user.Login, user.Name, user.Company, user.Location, user.Email,
			strconv.FormatBool(user.Hireable), user.Bio, strconv.Itoa(user.PublicRepos),
			strconv.Itoa(user.Followers), strconv.Itoa(user.Following), user.CreatedAt,
		})
	}
	return nil
}

func saveReposToCSV(repos []Repo) error {
	file, err := os.Create("repositories.csv")
	if err != nil {
		return err
	}
	defer file.Close()

	writer := csv.NewWriter(file)
	defer writer.Flush()

	writer.Write([]string{"login", "full_name", "created_at", "stargazers_count", "watchers_count", "language", "has_projects", "has_wiki", "license_name"})
	for _, repo := range repos {
		writer.Write([]string{
			repo.Login, repo.FullName, repo.CreatedAt,
			strconv.Itoa(repo.StargazersCount), strconv.Itoa(repo.WatchersCount),
			repo.Language, strconv.FormatBool(repo.HasProjects),
			strconv.FormatBool(repo.HasWiki), repo.LicenseName,
		})
	}
	return nil
}

func main() {
	users, err := fetchUsersInShanghai()
	if err != nil {
		fmt.Println("Error fetching users:", err)
		return
	}

	detailedUsers := fetchUserDetailsConcurrently(users)
	if err := saveUsersToCSV(detailedUsers); err != nil {
		fmt.Println("Error saving users to CSV:", err)
		return
	}

	allRepos := fetchUserReposConcurrently(detailedUsers)
	if err := saveReposToCSV(allRepos); err != nil {
		fmt.Println("Error saving repos to CSV:", err)
	}
	fmt.Println("Done")
}
