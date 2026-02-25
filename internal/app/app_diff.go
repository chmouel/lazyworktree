package app

import (
	tea "charm.land/bubbletea/v2"
	"github.com/chmouel/lazyworktree/internal/app/handlers"
	"github.com/chmouel/lazyworktree/internal/models"
)

func (m *Model) showDiff() tea.Cmd {
	if m.state.data.selectedIndex < 0 || m.state.data.selectedIndex >= len(m.state.data.filteredWts) {
		return nil
	}
	wt := m.state.data.filteredWts[m.state.data.selectedIndex]

	return m.diffRouter().ShowDiff(handlers.WorktreeDiffParams{
		Worktree:        wt,
		StatusFiles:     m.state.data.statusFilesAll,
		BuildCommandEnv: m.buildCommandEnv,
		ShowInfo: func(message string) {
			m.showInfo(message, nil)
		},
	})
}

// showFileDiff shows the diff for a single file in a pager.
func (m *Model) showFileDiff(sf StatusFile) tea.Cmd {
	if m.state.data.selectedIndex < 0 || m.state.data.selectedIndex >= len(m.state.data.filteredWts) {
		return nil
	}
	wt := m.state.data.filteredWts[m.state.data.selectedIndex]

	return m.diffRouter().ShowFileDiff(handlers.FileDiffParams{
		Worktree:        wt,
		File:            sf,
		BuildCommandEnv: m.buildCommandEnv,
	})
}

func (m *Model) showCommitDiff(commitSHA string, wt *models.WorktreeInfo) tea.Cmd {
	return m.diffRouter().ShowCommitDiff(handlers.CommitDiffParams{
		CommitSHA:       commitSHA,
		Worktree:        wt,
		BuildCommandEnv: m.buildCommandEnv,
	})
}

func (m *Model) showCommitFileDiff(commitSHA, filename, worktreePath string) tea.Cmd {
	return m.diffRouter().ShowCommitFileDiff(handlers.CommitFileDiffParams{
		CommitSHA:    commitSHA,
		Filename:     filename,
		WorktreePath: worktreePath,
	})
}

func (m *Model) diffRouter() *handlers.DiffRouter {
	return &handlers.DiffRouter{
		Config:                m.config,
		UseGitPager:           m.state.services.git.UseGitPager(),
		CommandRunner:         m.commandRunner,
		Context:               m.ctx,
		ExecProcess:           m.execProcess,
		PagerCommand:          m.pagerCommand,
		PagerEnv:              m.pagerEnv,
		FilterWorktreeEnvVars: filterWorktreeEnvVars,
		ShellQuote:            shellQuote,
		ErrorMsg: func(err error) tea.Msg {
			return errMsg{err: err}
		},
		RefreshMsg: func() tea.Msg {
			return refreshCompleteMsg{}
		},
	}
}
