import pytest
import json
from tests.utils import FakeGit, make_git_service


@pytest.mark.asyncio
async def test_detect_host_github_https() -> None:
    fake = FakeGit()
    fake.set(["git", "remote", "get-url", "origin"], "https://github.com/user/repo.git")

    svc = make_git_service(fake)
    host = await svc._detect_host()
    assert host == "github"


@pytest.mark.asyncio
async def test_detect_host_github_ssh() -> None:
    fake = FakeGit()
    fake.set(["git", "remote", "get-url", "origin"], "git@github.com:user/repo.git")

    svc = make_git_service(fake)
    host = await svc._detect_host()
    assert host == "github"


@pytest.mark.asyncio
async def test_detect_host_gitlab_ssh() -> None:
    fake = FakeGit()
    fake.set(["git", "remote", "get-url", "origin"], "git@gitlab.com:user/repo.git")

    svc = make_git_service(fake)
    host = await svc._detect_host()
    assert host == "gitlab"


@pytest.mark.asyncio
async def test_detect_host_gitlab_subdomain() -> None:
    fake = FakeGit()
    fake.set(
        ["git", "remote", "get-url", "origin"], "https://gitlab.gnome.org/GNOME/gtk.git"
    )

    svc = make_git_service(fake)
    host = await svc._detect_host()
    assert host == "gitlab"


@pytest.mark.asyncio
async def test_detect_host_enterprise_github() -> None:
    fake = FakeGit()
    fake.set(
        ["git", "remote", "get-url", "origin"], "https://github.ibm.com/org/repo.git"
    )

    svc = make_git_service(fake)
    host = await svc._detect_host()
    assert host == "github"


@pytest.mark.asyncio
async def test_detect_host_unknown() -> None:
    fake = FakeGit()
    fake.set(["git", "remote", "get-url", "origin"], "https://example.com/repo.git")

    svc = make_git_service(fake)
    host = await svc._detect_host()
    assert host == "unknown"


@pytest.mark.asyncio
async def test_fetch_pr_map_gitlab() -> None:
    fake = FakeGit()
    fake.set(["git", "remote", "get-url", "origin"], "git@gitlab.com:user/repo.git")

    glab_output = json.dumps(
        [
            {
                "source_branch": "feature",
                "state": "opened",
                "iid": 123,
                "title": "My MR",
                "web_url": "https://gitlab.com/user/repo/-/merge_requests/123",
            }
        ]
    )
    fake.set(["glab", "api", "merge_requests?state=all&per_page=100"], glab_output)

    svc = make_git_service(fake)
    pr_map = await svc.fetch_pr_map()

    assert pr_map is not None
    assert "feature" in pr_map
    pr = pr_map["feature"]
    assert pr.number == 123
    assert pr.state == "OPEN"
    assert pr.title == "My MR"
    assert pr.url == "https://gitlab.com/user/repo/-/merge_requests/123"


@pytest.mark.asyncio
async def test_fetch_pr_map_github() -> None:
    fake = FakeGit()
    fake.set(["git", "remote", "get-url", "origin"], "git@github.com:user/repo.git")

    gh_output = json.dumps(
        [
            {
                "headRefName": "feature",
                "state": "OPEN",
                "number": 123,
                "title": "My PR",
                "url": "https://github.com/user/repo/pull/123",
            }
        ]
    )
    fake.set(
        [
            "gh",
            "pr",
            "list",
            "--state",
            "all",
            "--json",
            "headRefName,state,number,title,url",
            "--limit",
            "100",
        ],
        gh_output,
    )

    svc = make_git_service(fake)
    pr_map = await svc.fetch_pr_map()

    assert pr_map is not None
    assert "feature" in pr_map
    pr = pr_map["feature"]
    assert pr.number == 123
    assert pr.state == "OPEN"
    assert pr.title == "My PR"
    assert pr.url == "https://github.com/user/repo/pull/123"
