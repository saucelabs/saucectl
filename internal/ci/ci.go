package ci

import (
	"fmt"
	"os"
)

type CI struct {
	Provider Provider
	URL      string
	Repo     string
	Ref      string // branch or tag
	ShortSHA string
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
	Gitpod = Provider{Name: "Gitpod", Envar: "GITPOD_WORKSPACE_ID"}
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
	Gitpod,
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

// GetCI returns the CI details if the code is executed in a known CI environment.
func GetCI(provider Provider) CI {
	switch provider {
	case AppVeyor:
		return CI{
			Provider: provider,
			URL:      fmt.Sprintf("%s/project/%s/%s/builds/%s", os.Getenv("APPVEYOR_URL"), os.Getenv("APPVEYOR_ACCOUNT_NAME"), os.Getenv("APPVEYOR_PROJECT_NAME"), os.Getenv("APPVEYOR_BUILD_ID")),
			Repo:     os.Getenv("APPVEYOR_REPO_NAME"),
			Ref:      os.Getenv("APPVEYOR_PULL_REQUEST_HEAD_REPO_BRANCH"),
			ShortSHA: shortenSHA(os.Getenv("APPVEYOR_REPO_COMMIT")),
		}
	case AWS:
		return CI{
			Provider: provider,
			URL:      os.Getenv("CODEBUILD_PUBLIC_BUILD_URL"),
			Repo:     os.Getenv("CODEBUILD_SOURCE_REPO_URL"),
			Ref:      os.Getenv("CODEBUILD_SOURCE_VERSION"),
			ShortSHA: shortenSHA(os.Getenv("CODEBUILD_RESOLVED_SOURCE_VERSION")),
		}
	case Azure:
		return CI{
			Provider: provider,
			URL:      os.Getenv("BUILD_REPOSITORY_URI"),
			Repo:     os.Getenv("SYSTEM_PULLREQUEST_SOURCEREPOSITORYURI"),
			Ref:      os.Getenv("BUILD_SOURCEBRANCHNAME"),
			ShortSHA: shortenSHA(os.Getenv("BUILD_SOURCEVERSION")),
		}
	case Bamboo:
		return CI{
			Provider: provider,
			URL:      os.Getenv("bamboo_resultsUrl"),
			Ref:      os.Getenv("bamboo_planRepository_branchDisplayName"),
		}
	case Bitbucket:
		return CI{
			Provider: provider,
			URL:      fmt.Sprintf("https://bitbucket.org/%s/addon/pipelines/home#!/results/%s", os.Getenv("BITBUCKET_REPO_FULL_NAME"), os.Getenv("BITBUCKET_BUILD_NUMBER")),
			Repo:     os.Getenv("BITBUCKET_REPO_FULL_NAME"),
			Ref:      os.Getenv("BITBUCKET_BRANCH"),
			ShortSHA: shortenSHA(os.Getenv("BITBUCKET_COMMIT")),
		}
	case Buildkite:
		return CI{
			Provider: provider,
			URL:      os.Getenv("BUILDKITE_BUILD_URL"),
			Repo:     os.Getenv("BUILDKITE_REPO"),
			Ref:      os.Getenv("BUILDKITE_BRANCH"),
			ShortSHA: shortenSHA(os.Getenv("BUILDKITE_COMMIT")),
		}
	case Buddy:
		return CI{
			Provider: provider,
			URL:      os.Getenv("BUDDY_PIPELINE_URL"),
			Repo:     os.Getenv("BUDDY_PROJECT_URL"),
			Ref:      os.Getenv("BUDDY_RUN_BRANCH"),
			ShortSHA: os.Getenv("BUDDY_RUN_COMMIT_SHORT"),
		}
	case Circle:
		return CI{
			Provider: provider,
			URL:      os.Getenv("CIRCLE_BUILD_URL"),
			Repo:     os.Getenv("CIRCLE_REPOSITORY_URL"),
			Ref:      os.Getenv("CIRCLE_BRANCH"),
			ShortSHA: shortenSHA(os.Getenv("CIRCLE_SHA1")),
		}
	case CodeShip:
		return CI{
			Provider: provider,
			URL:      os.Getenv("CI_BUILD_URL"),
			Repo:     os.Getenv("CI_REPO_NAME"),
			Ref:      os.Getenv("CI_BRANCH"),
			ShortSHA: shortenSHA(os.Getenv("CI_COMMIT_ID")),
		}
	case Drone:
		return CI{
			Provider: provider,
			URL:      os.Getenv("DRONE_BUILD_LINK"),
			Repo:     os.Getenv("DRONE_REPO"),
			Ref:      os.Getenv("DRONE_BRANCH"),
			ShortSHA: shortenSHA(os.Getenv("DRONE_COMMIT_SHA")),
		}
	case GitHub:
		return CI{
			Provider: provider,
			URL:      fmt.Sprintf("%s/%s/actions/runs/%s", os.Getenv("GITHUB_SERVER_URL"), os.Getenv("GITHUB_REPOSITORY"), os.Getenv("GITHUB_RUN_ID")),
			Repo:     os.Getenv("GITHUB_REPOSITORY"),
			Ref:      os.Getenv("GITHUB_REF_NAME"),
			ShortSHA: shortenSHA(os.Getenv("GITHUB_SHA")),
		}
	case GitLab:
		return CI{
			Provider: provider,
			URL:      os.Getenv("CI_JOB_URL"),
			Repo:     os.Getenv("CI_PROJECT_PATH"),
			Ref:      os.Getenv("CI_COMMIT_REF_NAME"),
			ShortSHA: os.Getenv("CI_COMMIT_SHORT_SHA"),
		}
	case Gitpod:
		return CI{
			Provider: provider,
			URL:      os.Getenv("GITPOD_WORKSPACE_URL"),
			Repo:     os.Getenv("GITPOD_REPO_ROOT"),
		}
	case Jenkins:
		return CI{
			Provider: provider,
			URL:      os.Getenv("JOB_URL"),
			Repo:     os.Getenv("GIT_URL"),
			Ref:      os.Getenv("GIT_BRANCH"),
			ShortSHA: shortenSHA(os.Getenv("GIT_COMMIT")),
		}
	case Semaphore:
		return CI{
			Provider: provider,
			URL:      fmt.Sprintf("%s/workflows/%s?pipeline_id=%s", os.Getenv("SEMAPHORE_ORGANIZATION_URL"), os.Getenv("SEMAPHORE_PROJECT_ID"), os.Getenv("SEMAPHORE_JOB_ID")),
			Repo:     os.Getenv("SEMAPHORE_GIT_URL"),
			Ref:      os.Getenv("SEMAPHORE_GIT_WORKING_BRANCH"),
			ShortSHA: shortenSHA(os.Getenv("SEMAPHORE_GIT_SHA")),
		}
	case Travis:
		return CI{
			Provider: provider,
			URL:      os.Getenv("TRAVIS_BUILD_WEB_URL"),
			Repo:     os.Getenv("TRAVIS_REPO_SLUG"),
			Ref:      os.Getenv("TRAVIS_BRANCH"),
			ShortSHA: shortenSHA(os.Getenv("TRAVIS_COMMIT")),
		}
	case TeamCity:
		return CI{
			Provider: provider,
		}
	}

	return CI{}
}

// GetTags returns tag list containing CI info.
func GetTags() []string {
	var tags []string
	provider := GetProvider()
	if provider == None {
		return tags
	}

	ci := GetCI(provider)
	tags = append(tags, ci.Provider.Name)
	if ci.URL != "" {
		tags = append(tags, fmt.Sprintf("%s:%s", "ci:url", ci.URL))
	}
	if ci.Repo != "" {
		tags = append(tags, fmt.Sprintf("%s:%s", "ci:repo", ci.Repo))
	}
	if ci.Ref != "" {
		tags = append(tags, fmt.Sprintf("%s:%s", "ci:ref", ci.Ref))
	}
	if ci.ShortSHA != "" {
		tags = append(tags, fmt.Sprintf("%s:%s", "ci:ssha", ci.ShortSHA))
	}

	return tags
}

// shortenSHA truncates a given SHA string to 7 characters.
// If the input SHA is already shorter than 8 characters, it is returned as-is without modification.
func shortenSHA(sha string) string {
	if len(sha) < 8 {
		return sha
	}

	return sha[:7]
}
