package ci

import (
	"fmt"
	"os"
	"reflect"
)

type CI struct {
	Provider  Provider
	OriginURL string
	Repo      string
	RefName   string // branch or tag
	SHA       string
	User      string
}

// Provider represents a CI Provider.
type Provider struct {
	// Name of the Provider.
	Name string

	// The environment variable by which the Provider is detected.
	Envar string
}

var (
	// AppVeyor represents https://www.appveyor.com/
	AppVeyor = Provider{Name: "AppVeyor", Envar: "APPVEYOR_BUILD_NUMBER"}
	// AWS represents https://aws.amazon.com/codebuild/
	AWS = Provider{Name: "AWS CodeBuild", Envar: "CODEBUILD_INITIATOR"}
	// Azure represents https://azure.microsoft.com/en-us/services/devops/
	Azure = Provider{Name: "Azure DevOps", Envar: "Agent_BuildDirectory"}
	// Bamboo represents https://www.atlassian.com/software/bamboo
	Bamboo = Provider{Name: "Bamboo", Envar: "bamboo_buildNumber"}
	// Bitbucket represents https://bitbucket.org/product/features/pipelines
	Bitbucket = Provider{Name: "Bitbucket", Envar: "BITBUCKET_BUILD_NUMBER"}
	// Buildkite represents https://buildkite.com/
	Buildkite = Provider{Name: "Buildkite", Envar: "BUILDKITE"}
	// Buddy represents https://buddy.works/
	Buddy = Provider{Name: "Buddy", Envar: "BUDDY"}
	// Circle represents https://circleci.com/
	Circle = Provider{Name: "CircleCI", Envar: "CIRCLECI"}
	// CodeShip represents https://www.cloudbees.com/products/codeship
	CodeShip = Provider{Name: "CloudBees CodeShip", Envar: "CI_NAME"}
	// Drone represents https://www.drone.io/
	Drone = Provider{Name: "Drone", Envar: "DRONE_BUILD_NUMBER"}
	// GitHub represents https://github.com/
	GitHub = Provider{Name: "GitHub", Envar: "GITHUB_RUN_ID"}
	// GitLab represents https://about.gitlab.com/
	GitLab = Provider{Name: "GitLab", Envar: "CI_PIPELINE_ID"}
	// Jenkins represents https://www.jenkins.io/
	Jenkins = Provider{Name: "Jenkins", Envar: "BUILD_NUMBER"}
	// Semaphore represents https://semaphoreci.com/
	Semaphore = Provider{Name: "Semaphore", Envar: "SEMAPHORE_EXECUTABLE_UUID"}
	// Travis represents https://www.travis-ci.com/
	Travis = Provider{Name: "Travis CI", Envar: "TRAVIS_BUILD_ID"}
	// TeamCity represents https://www.jetbrains.com/teamcity/
	TeamCity = Provider{Name: "TeamCity", Envar: "TEAMCITY_VERSION"}

	// None represents a non-CI environment.
	None = Provider{}
)

// Providers contains a list of all supported providers.
var Providers = []Provider{
	AppVeyor,
	AWS,
	Azure,
	Bamboo,
	Bitbucket,
	Buildkite,
	Buddy,
	Circle,
	CodeShip,
	Drone,
	GitHub,
	GitLab,
	Jenkins,
	Semaphore,
	Travis,
	TeamCity,
}

// GetProvider returns a CI Provider if this code is executed in a known CI environment.
// Returns None if it's not a CI environment or if the CI Provider could not be detected.
func GetProvider() Provider {
	for _, p := range Providers {
		_, ok := os.LookupEnv(p.Envar)
		if ok {
			return p
		}
	}

	return None
}

func getCI(provider Provider) CI {
	var ci CI
	if reflect.DeepEqual(provider, AppVeyor) {
		ci = CI{
			Provider:  provider,
			OriginURL: os.Getenv(fmt.Sprintf("%s/project/%s/%s/builds/%s", os.Getenv("APPVEYOR_URL"), os.Getenv("APPVEYOR_ACCOUNT_NAME"), os.Getenv("APPVEYOR_PROJECT_NAME"), os.Getenv("APPVEYOR_BUILD_ID"))),
			Repo:      os.Getenv("APPVEYOR_REPO_NAME"),
			RefName:   os.Getenv("APPVEYOR_PULL_REQUEST_HEAD_REPO_BRANCH"),
			SHA:       os.Getenv("APPVEYOR_REPO_COMMIT"),
		}
	}
	if reflect.DeepEqual(provider, AWS) {
		ci = CI{
			Provider:  provider,
			OriginURL: os.Getenv("CODEBUILD_PUBLIC_BUILD_URL"),
			Repo:      os.Getenv("CODEBUILD_SOURCE_REPO_URL"),
			RefName:   os.Getenv("CODEBUILD_SOURCE_VERSION"),
			SHA:       os.Getenv("CODEBUILD_RESOLVED_SOURCE_VERSION"),
			User:      os.Getenv("CODEBUILD_WEBHOOK_ACTOR_ACCOUNT_ID"),
		}
	}
	if reflect.DeepEqual(provider, Azure) {
		ci = CI{
			Provider:  provider,
			OriginURL: os.Getenv("Build_BuildUri"),
			Repo:      os.Getenv("System_PullRequest_SourceRepositoryURI"),
			RefName:   os.Getenv("Build_SourceBranchName"),
			SHA:       os.Getenv("Build_SourceVersion"),
			User:      os.Getenv("Build_RequestedFor"),
		}
	}
	if reflect.DeepEqual(provider, Bamboo) {
		ci = CI{
			Provider:  provider,
			OriginURL: os.Getenv("bamboo_buildResultsUrl"),
			Repo:      os.Getenv("bamboo_planRepository_repositoryUrl"),
			RefName:   os.Getenv("bamboo_planRepository_branch"),
			SHA:       os.Getenv("bamboo_planRepository_revision"),
			User:      os.Getenv("bamboo_ManualBuildTriggerReason_userName"),
		}
	}
	if reflect.DeepEqual(provider, Bitbucket) {
		ci = CI{
			Provider: provider,
			Repo:     os.Getenv("BITBUCKET_REPO_FULL_NAME"),
			RefName:  os.Getenv("BITBUCKET_BRANCH"),
			SHA:      os.Getenv("BITBUCKET_COMMIT"),
			User:     os.Getenv("BITBUCKET_STEP_TRIGGERER_UUID"),
		}
	}
	if reflect.DeepEqual(provider, Buildkite) {
		ci = CI{
			Provider:  provider,
			OriginURL: os.Getenv("BUILDKITE_BUILD_URL"),
			Repo:      os.Getenv("BUILDKITE_REPO"),
			RefName:   os.Getenv("BUILDKITE_BRANCH"),
			SHA:       os.Getenv("BUILDKITE_COMMIT"),
			User:      os.Getenv("BUILDKITE_BUILD_CREATOR"),
		}
	}
	if reflect.DeepEqual(provider, Buddy) {
		ci = CI{
			Provider:  provider,
			OriginURL: os.Getenv("BUDDY_PIPELINE_URL"),
			Repo:      os.Getenv("BUDDY_PROJECT_URL"),
			RefName:   os.Getenv("BUDDY_EXECUTION_BRANCH"),
			SHA:       os.Getenv("BUDDY_EXECUTION_REVISION"),
			User:      os.Getenv("BUDDY_INVOKER_NAME"),
		}
	}
	if reflect.DeepEqual(provider, Circle) {
		ci = CI{
			Provider:  provider,
			OriginURL: os.Getenv("CIRCLE_BUILD_URL"),
			Repo:      os.Getenv("CIRCLE_REPOSITORY_URL"),
			RefName:   os.Getenv("CIRCLE_BRANCH"),
			SHA:       os.Getenv("CIRCLE_SHA1"),
			User:      os.Getenv("CIRCLE_USERNAME"),
		}
	}
	if reflect.DeepEqual(provider, CodeShip) {
		ci = CI{
			Provider:  provider,
			OriginURL: os.Getenv("CI_BUILD_URL"),
			Repo:      os.Getenv("CI_REPO_NAME"),
			RefName:   os.Getenv("CI_BRANCH"),
			SHA:       os.Getenv("CI_COMMIT_ID"),
		}
	}
	if reflect.DeepEqual(provider, Drone) {
		ci = CI{
			Provider:  provider,
			OriginURL: os.Getenv("DRONE_BUILD_LINK"),
			Repo:      os.Getenv("DRONE_REPO"),
			RefName:   os.Getenv("DRONE_BRANCH"),
			SHA:       os.Getenv("DRONE_COMMIT_SHA"),
		}
	}
	if reflect.DeepEqual(provider, GitHub) {
		ci = CI{
			Provider:  provider,
			OriginURL: fmt.Sprintf("%s/%s/actions/runs/%s", os.Getenv("GITHUB_SERVER_URL"), os.Getenv("GITHUB_REPOSITORY"), os.Getenv("GITHUB_RUN_ID")),
			Repo:      os.Getenv("GITHUB_REPOSITORY"),
			RefName:   os.Getenv("GITHUB_REF_NAME"),
			SHA:       os.Getenv("GITHUB_SHA"),
			User:      os.Getenv("GITHUB_ACTOR"),
		}
	}
	if reflect.DeepEqual(provider, GitLab) {
		ci = CI{
			Provider:  provider,
			OriginURL: os.Getenv("CI_JOB_URL"),
			Repo:      os.Getenv("CI_PROJECT_PATH"),
			RefName:   os.Getenv("CI_COMMIT_REF_NAME"),
			SHA:       os.Getenv("CI_COMMIT_SHA"),
			User:      os.Getenv("GITLAB_USER_LOGIN"),
		}
	}

	if reflect.DeepEqual(provider, Jenkins) {
		ci = CI{
			Provider:  provider,
			OriginURL: os.Getenv("JOB_URL"),
			Repo:      os.Getenv("GIT_URL"),
			RefName:   os.Getenv("GIT_BRANCH"),
			SHA:       os.Getenv("GIT_COMMIT"),
		}
	}
	if reflect.DeepEqual(provider, Semaphore) {
		ci = CI{
			Provider: provider,
			Repo:     os.Getenv("SEMAPHORE_GIT_URL"),
			RefName:  os.Getenv("SEMAPHORE_GIT_BRANCH"),
			SHA:      os.Getenv("SEMAPHORE_GIT_SHA"),
		}
	}
	if reflect.DeepEqual(provider, Travis) {
		ci = CI{
			Provider:  provider,
			OriginURL: os.Getenv("TRAVIS_BUILD_WEB_URL"),
			Repo:      os.Getenv("TRAVIS_REPO_SLUG"),
			RefName:   os.Getenv("TRAVIS_BRANCH"),
			SHA:       os.Getenv("TRAVIS_COMMIT"),
		}
	}
	if reflect.DeepEqual(provider, TeamCity) {
		ci = CI{
			Provider: provider,
		}
	}

	return ci
}

// GetTags returns tag list containing CI info
func GetTags() []string {
	provider := GetProvider()
	if reflect.DeepEqual(provider, None) {
		return []string{}
	}

	ci := getCI(provider)
	return []string{
		ci.Provider.Name,
		fmt.Sprintf("%s:%s", "originURL", ci.OriginURL),
		fmt.Sprintf("%s:%s", "repo", ci.Repo),
		fmt.Sprintf("%s:%s", "refName", ci.RefName),
		fmt.Sprintf("%s:%s", "SHA", ci.SHA),
		fmt.Sprintf("%s:%s", "user", ci.User),
	}
}
