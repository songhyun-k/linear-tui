package tui

import (
	"context"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
	"github.com/roeyazroel/linear-tui/internal/config"
	"github.com/roeyazroel/linear-tui/internal/linearapi"
)

// stringPtr returns a string pointer for test helpers.
func stringPtr(value string) *string {
	return &value
}

// waitForCondition polls until a condition is true or times out.
func waitForCondition(t *testing.T, timeout time.Duration, check func() bool) {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if check() {
			return
		}
		time.Sleep(5 * time.Millisecond)
	}
	t.Fatalf("condition not met within %s", timeout)
}

func TestTogglePaneZoom_TogglesFocusedPaneZoomAndRestore(t *testing.T) {
	tests := []struct {
		name        string
		focusedPane FocusTarget
	}{
		{name: "navigation", focusedPane: FocusNavigation},
		{name: "issues", focusedPane: FocusIssues},
		{name: "details", focusedPane: FocusDetails},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			app := NewApp(&linearapi.Client{}, config.Config{}, nil)
			app.focusedPane = tt.focusedPane

			if !app.togglePaneZoom() {
				t.Fatal("zoom togglePaneZoom() = false, want true")
			}
			if !app.paneZoomed {
				t.Fatal("paneZoomed = false, want true")
			}
			if app.zoomedPane != tt.focusedPane {
				t.Fatalf("zoomedPane = %v, want %v", app.zoomedPane, tt.focusedPane)
			}
			if app.previousFocusedPane != tt.focusedPane {
				t.Fatalf("previousFocusedPane = %v, want %v", app.previousFocusedPane, tt.focusedPane)
			}

			if !app.togglePaneZoom() {
				t.Fatal("restore togglePaneZoom() = false, want true")
			}
			if app.paneZoomed {
				t.Fatal("paneZoomed = true, want false")
			}
			if app.zoomedPane != FocusNone {
				t.Fatalf("zoomedPane = %v, want %v", app.zoomedPane, FocusNone)
			}
			if app.previousFocusedPane != FocusNone {
				t.Fatalf("previousFocusedPane = %v, want %v", app.previousFocusedPane, FocusNone)
			}
			if app.focusedPane != tt.focusedPane {
				t.Fatalf("focusedPane = %v, want %v", app.focusedPane, tt.focusedPane)
			}
			if app.mainContent.GetItemCount() != 3 {
				t.Fatalf("restored pane count = %d, want 3", app.mainContent.GetItemCount())
			}
			if app.mainContent.GetItem(0) != app.navigationTree {
				t.Fatal("restored first pane is not navigation")
			}
			if app.mainContent.GetItem(1) != app.issuesColumn {
				t.Fatal("restored second pane is not issues")
			}
			if app.mainContent.GetItem(2) != app.detailsView {
				t.Fatal("restored third pane is not details")
			}
		})
	}
}

func TestTogglePaneZoom_IgnoresNonPaneFocusWhenNotZoomed(t *testing.T) {
	app := NewApp(&linearapi.Client{}, config.Config{}, nil)
	app.focusedPane = FocusPalette
	app.zoomedPane = FocusNone
	app.previousFocusedPane = FocusNone

	if app.togglePaneZoom() {
		t.Fatal("togglePaneZoom() = true, want false")
	}
	if app.paneZoomed {
		t.Fatal("paneZoomed = true, want false")
	}
	if app.zoomedPane != FocusNone {
		t.Fatalf("zoomedPane = %v, want %v", app.zoomedPane, FocusNone)
	}
}

func TestRestorePaneZoom_ReturnsFocusToPaneFocusedBeforeZooming(t *testing.T) {
	tests := []struct {
		name                        string
		focusedBeforeZoom           FocusTarget
		transientFocusedWhileZoomed FocusTarget
		wantPrimitive               func(*App) tview.Primitive
	}{
		{
			name:                        "navigation",
			focusedBeforeZoom:           FocusNavigation,
			transientFocusedWhileZoomed: FocusDetails,
			wantPrimitive:               func(app *App) tview.Primitive { return app.navigationTree },
		},
		{
			name:                        "issues",
			focusedBeforeZoom:           FocusIssues,
			transientFocusedWhileZoomed: FocusNavigation,
			wantPrimitive:               func(app *App) tview.Primitive { return app.otherIssuesTable },
		},
		{
			name:                        "details",
			focusedBeforeZoom:           FocusDetails,
			transientFocusedWhileZoomed: FocusIssues,
			wantPrimitive:               func(app *App) tview.Primitive { return app.detailsDescriptionView },
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			app := NewApp(&linearapi.Client{}, config.Config{}, nil)
			app.focusedPane = tt.focusedBeforeZoom

			if !app.togglePaneZoom() {
				t.Fatal("zoom togglePaneZoom() = false, want true")
			}

			app.focusedPane = tt.transientFocusedWhileZoomed

			if !app.togglePaneZoom() {
				t.Fatal("restore togglePaneZoom() = false, want true")
			}
			if app.focusedPane != tt.focusedBeforeZoom {
				t.Fatalf("focusedPane = %v, want %v", app.focusedPane, tt.focusedBeforeZoom)
			}
			if got, want := app.app.GetFocus(), tt.wantPrimitive(app); got != want {
				t.Fatalf("application focus = %T, want %T", got, want)
			}
		})
	}
}

func TestUpdatePaneLayout_ZoomedPaneSkipsNonFocusedPanes(t *testing.T) {
	tests := []struct {
		name        string
		focusedPane FocusTarget
		wantPane    func(*App) interface{}
	}{
		{
			name:        "navigation",
			focusedPane: FocusNavigation,
			wantPane:    func(app *App) interface{} { return app.navigationTree },
		},
		{
			name:        "issues",
			focusedPane: FocusIssues,
			wantPane:    func(app *App) interface{} { return app.issuesColumn },
		},
		{
			name:        "details",
			focusedPane: FocusDetails,
			wantPane:    func(app *App) interface{} { return app.detailsView },
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			app := NewApp(&linearapi.Client{}, config.Config{}, nil)
			if app.mainContent.GetItemCount() != 3 {
				t.Fatalf("normal pane count = %d, want 3", app.mainContent.GetItemCount())
			}

			app.focusedPane = tt.focusedPane
			if !app.togglePaneZoom() {
				t.Fatal("togglePaneZoom() = false, want true")
			}

			if app.mainContent.GetItemCount() != 1 {
				t.Fatalf("zoomed pane count = %d, want 1", app.mainContent.GetItemCount())
			}
			if got, want := app.mainContent.GetItem(0), tt.wantPane(app); got != want {
				t.Fatalf("zoomed pane = %T, want %T", got, want)
			}

			app.restorePaneZoom()
			if app.mainContent.GetItemCount() != 3 {
				t.Fatalf("restored pane count = %d, want 3", app.mainContent.GetItemCount())
			}
			if app.mainContent.GetItem(0) != app.navigationTree {
				t.Fatal("restored first pane is not navigation")
			}
			if app.mainContent.GetItem(1) != app.issuesColumn {
				t.Fatal("restored second pane is not issues")
			}
			if app.mainContent.GetItem(2) != app.detailsView {
				t.Fatal("restored third pane is not details")
			}
		})
	}
}

func TestUpdatePaneLayout_ZoomedPanePreservesPaneChrome(t *testing.T) {
	tests := []struct {
		name        string
		focusedPane FocusTarget
		wantTitle   string
	}{
		{
			name:        "navigation",
			focusedPane: FocusNavigation,
			wantTitle:   "Navigation",
		},
		{
			name:        "issues",
			focusedPane: FocusIssues,
			wantTitle:   "Other Issues",
		},
		{
			name:        "details",
			focusedPane: FocusDetails,
			wantTitle:   "Details",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			app := NewApp(&linearapi.Client{}, config.Config{}, nil)
			app.focusedPane = tt.focusedPane
			if !app.togglePaneZoom() {
				t.Fatal("togglePaneZoom() = false, want true")
			}

			rendered := renderPrimitiveForTest(t, app.mainContent, 80, 20)
			if !strings.Contains(rendered, tt.wantTitle) {
				t.Fatalf("zoomed pane render does not include title %q:\n%s", tt.wantTitle, rendered)
			}
			if !containsAnyRune(rendered, "┌┐└┘─│╔╗╚╝═║") {
				t.Fatalf("zoomed pane render does not include border characters:\n%s", rendered)
			}
		})
	}
}

func renderPrimitiveForTest(t *testing.T, primitive tview.Primitive, width, height int) string {
	t.Helper()

	screen := tcell.NewSimulationScreen("")
	if err := screen.Init(); err != nil {
		t.Fatalf("screen.Init() error = %v", err)
	}
	defer screen.Fini()

	screen.SetSize(width, height)
	primitive.SetRect(0, 0, width, height)
	primitive.Draw(screen)

	var builder strings.Builder
	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			mainc, _, _, _ := screen.GetContent(x, y)
			if mainc == 0 {
				mainc = ' '
			}
			builder.WriteRune(mainc)
		}
		builder.WriteByte('\n')
	}
	return builder.String()
}

func containsAnyRune(value, runes string) bool {
	for _, r := range runes {
		if strings.ContainsRune(value, r) {
			return true
		}
	}
	return false
}

func TestDefaultCommands_DoNotShadowZoomShortcut(t *testing.T) {
	const zoomShortcut = 'z'

	for _, cmd := range DefaultCommands(nil) {
		if cmd.ShortcutRune == zoomShortcut {
			t.Fatalf("command %q uses %q, which is reserved for pane zoom/restore", cmd.ID, zoomShortcut)
		}
		if strings.EqualFold(strings.TrimSpace(cmd.ShortcutDisplay), string(zoomShortcut)) {
			t.Fatalf("command %q displays %q, which is reserved for pane zoom/restore", cmd.ID, cmd.ShortcutDisplay)
		}
	}
}

func TestPaneShortcuts_DoNotShadowZoomShortcut(t *testing.T) {
	const zoomShortcut = 'z'

	paneShortcuts := map[string][]rune{
		"navigation":  {'l'},
		"issues":      {'h', 'l'},
		"issuesTable": {'j', 'k', 'g', 'G', 'h', 'l', ' '},
		"details":     {'h'},
	}

	for pane, shortcuts := range paneShortcuts {
		for _, shortcut := range shortcuts {
			if shortcut == zoomShortcut {
				t.Fatalf("%s pane shortcut %q conflicts with pane zoom/restore", pane, zoomShortcut)
			}
		}
	}
}

func TestPaneHandlersClaimZoomShortcut(t *testing.T) {
	tests := []struct {
		name        string
		focusedPane FocusTarget
		handle      func(*App, *tcell.EventKey) *tcell.EventKey
	}{
		{
			name:        "navigation",
			focusedPane: FocusNavigation,
			handle:      (*App).handleNavigationKey,
		},
		{
			name:        "issues",
			focusedPane: FocusIssues,
			handle:      (*App).handleIssuesKey,
		},
		{
			name:        "details",
			focusedPane: FocusDetails,
			handle:      (*App).handleDetailsKey,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			app := NewApp(&linearapi.Client{}, config.Config{}, nil)
			app.focusedPane = tt.focusedPane
			event := tcell.NewEventKey(tcell.KeyRune, 'z', tcell.ModNone)

			if returned := tt.handle(app, event); returned != nil {
				t.Fatalf("handler returned %v, want nil", returned)
			}
			if !app.paneZoomed {
				t.Fatal("paneZoomed = false, want true")
			}
			if app.zoomedPane != tt.focusedPane {
				t.Fatalf("zoomedPane = %v, want %v", app.zoomedPane, tt.focusedPane)
			}

			if returned := tt.handle(app, event); returned != nil {
				t.Fatalf("restore handler returned %v, want nil", returned)
			}
			if app.paneZoomed {
				t.Fatal("paneZoomed = true, want false")
			}
			if app.zoomedPane != FocusNone {
				t.Fatalf("zoomedPane = %v, want %v", app.zoomedPane, FocusNone)
			}
			if app.previousFocusedPane != FocusNone {
				t.Fatalf("previousFocusedPane = %v, want %v", app.previousFocusedPane, FocusNone)
			}
			if app.focusedPane != tt.focusedPane {
				t.Fatalf("focusedPane = %v, want %v", app.focusedPane, tt.focusedPane)
			}
			if app.mainContent.GetItemCount() != 3 {
				t.Fatalf("restored pane count = %d, want 3", app.mainContent.GetItemCount())
			}
			if app.mainContent.GetItem(0) != app.navigationTree {
				t.Fatal("restored first pane is not navigation")
			}
			if app.mainContent.GetItem(1) != app.issuesColumn {
				t.Fatal("restored second pane is not issues")
			}
			if app.mainContent.GetItem(2) != app.detailsView {
				t.Fatal("restored third pane is not details")
			}
		})
	}
}

func TestZoomedIssuesPane_RunsCommandShortcuts(t *testing.T) {
	app := NewApp(&linearapi.Client{}, config.Config{}, nil)
	app.focusedPane = FocusIssues
	app.paneZoomed = true
	app.zoomedPane = FocusIssues
	app.previousFocusedPane = FocusIssues

	runCount := 0
	app.paletteCtrl.commands = []Command{
		{
			ID:           "test_command",
			ShortcutRune: 'r',
			Run: func(a *App) {
				runCount++
			},
		},
	}

	event := tcell.NewEventKey(tcell.KeyRune, 'r', tcell.ModNone)
	if returned := app.handleIssuesKey(event); returned != nil {
		t.Fatalf("handleIssuesKey returned %v, want nil", returned)
	}
	if runCount != 1 {
		t.Fatalf("command run count = %d, want 1", runCount)
	}
	if !app.paneZoomed {
		t.Fatal("paneZoomed = false, want true")
	}
	if app.zoomedPane != FocusIssues {
		t.Fatalf("zoomedPane = %v, want %v", app.zoomedPane, FocusIssues)
	}
	if app.focusedPane != FocusIssues {
		t.Fatalf("focusedPane = %v, want %v", app.focusedPane, FocusIssues)
	}
}

func TestZoomedIssuesPane_PreservesTableNavigation(t *testing.T) {
	app := NewApp(&linearapi.Client{}, config.Config{}, nil)
	app.queueUpdateDraw = func(f func()) { f() }

	issue1 := linearapi.Issue{ID: "issue-1", Identifier: "ABC-1", Title: "First", State: "Todo"}
	issue2 := linearapi.Issue{ID: "issue-2", Identifier: "ABC-2", Title: "Second", State: "Todo"}
	issueByID := map[string]linearapi.Issue{
		issue1.ID: issue1,
		issue2.ID: issue2,
	}
	app.fetchIssueByID = func(ctx context.Context, id string) (linearapi.Issue, error) {
		return issueByID[id], nil
	}
	app.updateIssuesData([]linearapi.Issue{issue1, issue2}, issue1.ID)

	app.focusedPane = FocusIssues
	app.activeIssuesSection = IssuesSectionOther
	app.paneZoomed = true
	app.zoomedPane = FocusIssues
	app.previousFocusedPane = FocusIssues
	app.updateFocus()

	event := tcell.NewEventKey(tcell.KeyRune, 'j', tcell.ModNone)
	returned := app.handleIssuesKey(event)
	if returned != event {
		t.Fatalf("handleIssuesKey returned %v, want original event", returned)
	}

	handler := app.otherIssuesTable.InputHandler()
	handler(returned, nil)

	row, _ := app.otherIssuesTable.GetSelection()
	if row != 2 {
		t.Fatalf("selected row = %d, want 2", row)
	}
	if app.selectedIssue == nil || app.selectedIssue.ID != issue2.ID {
		t.Fatalf("selectedIssue = %#v, want %s", app.selectedIssue, issue2.ID)
	}
	if !app.paneZoomed {
		t.Fatal("paneZoomed = false, want true")
	}
	if app.zoomedPane != FocusIssues {
		t.Fatalf("zoomedPane = %v, want %v", app.zoomedPane, FocusIssues)
	}
	if app.focusedPane != FocusIssues {
		t.Fatalf("focusedPane = %v, want %v", app.focusedPane, FocusIssues)
	}
}

func TestZoomedIssuesPane_EnterKeepsVisiblePaneInSyncWithFocus(t *testing.T) {
	app := NewApp(&linearapi.Client{}, config.Config{}, nil)
	app.queueUpdateDraw = func(f func()) { f() }

	issue := linearapi.Issue{ID: "issue-1", Identifier: "ABC-1", Title: "Leaf", State: "Todo"}
	app.fetchIssueByID = func(ctx context.Context, id string) (linearapi.Issue, error) {
		return issue, nil
	}
	app.updateIssuesData([]linearapi.Issue{issue}, issue.ID)

	app.focusedPane = FocusIssues
	app.activeIssuesSection = IssuesSectionOther
	if !app.togglePaneZoom() {
		t.Fatal("togglePaneZoom() = false, want true")
	}

	handler := app.otherIssuesTable.InputHandler()
	handler(tcell.NewEventKey(tcell.KeyEnter, 0, tcell.ModNone), nil)

	if !app.paneZoomed {
		t.Fatal("paneZoomed = false, want true")
	}
	if app.focusedPane != FocusDetails {
		t.Fatalf("focusedPane = %v, want %v", app.focusedPane, FocusDetails)
	}
	if app.zoomedPane != FocusDetails {
		t.Fatalf("zoomedPane = %v, want %v", app.zoomedPane, FocusDetails)
	}
	if app.mainContent.GetItemCount() != 1 {
		t.Fatalf("zoomed pane count = %d, want 1", app.mainContent.GetItemCount())
	}
	if app.mainContent.GetItem(0) != app.detailsView {
		t.Fatal("zoomed pane is not details")
	}
}

func TestZoomedPane_ClosePaletteRestoresZoomedPaneFocus(t *testing.T) {
	app := NewApp(&linearapi.Client{}, config.Config{}, nil)
	app.focusedPane = FocusDetails
	if !app.togglePaneZoom() {
		t.Fatal("togglePaneZoom() = false, want true")
	}

	app.openPalette()
	app.closePalette()

	if app.focusedPane != FocusDetails {
		t.Fatalf("focusedPane = %v, want %v", app.focusedPane, FocusDetails)
	}
	if app.zoomedPane != FocusDetails {
		t.Fatalf("zoomedPane = %v, want %v", app.zoomedPane, FocusDetails)
	}
	if app.mainContent.GetItemCount() != 1 {
		t.Fatalf("zoomed pane count = %d, want 1", app.mainContent.GetItemCount())
	}
	if app.mainContent.GetItem(0) != app.detailsView {
		t.Fatal("zoomed pane is not details")
	}
}

func TestZoomedPaneSwitchKeysAreIgnored(t *testing.T) {
	tests := []struct {
		name  string
		event *tcell.EventKey
	}{
		{name: "tab", event: tcell.NewEventKey(tcell.KeyTab, 0, tcell.ModNone)},
		{name: "shift tab key", event: tcell.NewEventKey(tcell.KeyBacktab, 0, tcell.ModShift)},
		{name: "shift tab modifier", event: tcell.NewEventKey(tcell.KeyTab, 0, tcell.ModShift)},
		{name: "left arrow", event: tcell.NewEventKey(tcell.KeyLeft, 0, tcell.ModNone)},
		{name: "right arrow", event: tcell.NewEventKey(tcell.KeyRight, 0, tcell.ModNone)},
		{name: "h", event: tcell.NewEventKey(tcell.KeyRune, 'h', tcell.ModNone)},
		{name: "l", event: tcell.NewEventKey(tcell.KeyRune, 'l', tcell.ModNone)},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			app := NewApp(&linearapi.Client{}, config.Config{}, nil)
			app.focusedPane = FocusIssues
			app.paneZoomed = true
			app.zoomedPane = FocusIssues
			app.previousFocusedPane = FocusIssues

			if returned := app.handleGlobalKey(tt.event); returned != nil {
				t.Fatalf("handleGlobalKey returned %v, want nil", returned)
			}
			if app.focusedPane != FocusIssues {
				t.Fatalf("focusedPane = %v, want %v", app.focusedPane, FocusIssues)
			}
			if !app.paneZoomed {
				t.Fatal("paneZoomed = false, want true")
			}
			if app.zoomedPane != FocusIssues {
				t.Fatalf("zoomedPane = %v, want %v", app.zoomedPane, FocusIssues)
			}
		})
	}
}

func TestPaneSwitchKeysStillSwitchWhenNotZoomed(t *testing.T) {
	tests := []struct {
		name      string
		event     *tcell.EventKey
		wantFocus FocusTarget
	}{
		{name: "tab", event: tcell.NewEventKey(tcell.KeyTab, 0, tcell.ModNone), wantFocus: FocusDetails},
		{name: "shift tab key", event: tcell.NewEventKey(tcell.KeyBacktab, 0, tcell.ModShift), wantFocus: FocusNavigation},
		{name: "shift tab modifier", event: tcell.NewEventKey(tcell.KeyTab, 0, tcell.ModShift), wantFocus: FocusNavigation},
		{name: "left arrow", event: tcell.NewEventKey(tcell.KeyLeft, 0, tcell.ModNone), wantFocus: FocusNavigation},
		{name: "right arrow", event: tcell.NewEventKey(tcell.KeyRight, 0, tcell.ModNone), wantFocus: FocusDetails},
		{name: "h", event: tcell.NewEventKey(tcell.KeyRune, 'h', tcell.ModNone), wantFocus: FocusNavigation},
		{name: "l", event: tcell.NewEventKey(tcell.KeyRune, 'l', tcell.ModNone), wantFocus: FocusDetails},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			app := NewApp(&linearapi.Client{}, config.Config{}, nil)
			app.focusedPane = FocusIssues

			if returned := app.handleGlobalKey(tt.event); returned != nil {
				t.Fatalf("handleGlobalKey returned %v, want nil", returned)
			}
			if app.focusedPane != tt.wantFocus {
				t.Fatalf("focusedPane = %v, want %v", app.focusedPane, tt.wantFocus)
			}
		})
	}
}

// TestRefreshIssues_LazyLoadsPages verifies first page renders before background pages.
func TestRefreshIssues_LazyLoadsPages(t *testing.T) {
	cfg := config.Config{
		PageSize: 2,
		CacheTTL: time.Minute,
	}
	app := NewApp(&linearapi.Client{}, cfg, nil)
	app.queueUpdateDraw = func(f func()) { f() }

	issue1 := linearapi.Issue{ID: "issue-1", Identifier: "ABC-1", Title: "First", State: "Todo"}
	issue2 := linearapi.Issue{ID: "issue-2", Identifier: "ABC-2", Title: "Second", State: "Todo"}

	issueByID := map[string]linearapi.Issue{
		issue1.ID: issue1,
		issue2.ID: issue2,
	}
	app.fetchIssueByID = func(ctx context.Context, id string) (linearapi.Issue, error) {
		return issueByID[id], nil
	}

	blockNext := make(chan struct{})
	app.fetchIssuesPage = func(ctx context.Context, params linearapi.FetchIssuesParams, after *string) (linearapi.IssuePage, error) {
		if after == nil {
			return linearapi.IssuePage{
				Issues:    []linearapi.Issue{issue1},
				HasNext:   true,
				EndCursor: stringPtr("cursor-1"),
			}, nil
		}
		<-blockNext
		return linearapi.IssuePage{
			Issues:  []linearapi.Issue{issue2},
			HasNext: false,
		}, nil
	}

	app.refreshIssues()

	waitForCondition(t, time.Second, func() bool {
		app.issuesMu.RLock()
		defer app.issuesMu.RUnlock()
		return len(app.issues) == 1
	})
	app.issuesMu.RLock()
	selectedIssue := app.selectedIssue
	app.issuesMu.RUnlock()
	if selectedIssue == nil || selectedIssue.ID != issue1.ID {
		t.Fatalf("selectedIssue = %#v, want %s", selectedIssue, issue1.ID)
	}

	close(blockNext)
	waitForCondition(t, time.Second, func() bool {
		app.issuesMu.RLock()
		defer app.issuesMu.RUnlock()
		return len(app.issues) == 2
	})
	app.issuesMu.RLock()
	selectedIssue = app.selectedIssue
	app.issuesMu.RUnlock()
	if selectedIssue == nil || selectedIssue.ID != issue1.ID {
		t.Fatalf("selectedIssue after append = %#v, want %s", selectedIssue, issue1.ID)
	}
}

// TestRefreshIssues_CancelsStaleLoad verifies stale background pages are ignored.
func TestRefreshIssues_CancelsStaleLoad(t *testing.T) {
	cfg := config.Config{
		PageSize: 2,
		CacheTTL: time.Minute,
	}
	app := NewApp(&linearapi.Client{}, cfg, nil)
	app.queueUpdateDraw = func(f func()) { f() }

	issue1 := linearapi.Issue{ID: "issue-1", Identifier: "ABC-1", Title: "First", State: "Todo"}
	issue2 := linearapi.Issue{ID: "issue-2", Identifier: "ABC-2", Title: "Second", State: "Todo"}
	issue3 := linearapi.Issue{ID: "issue-3", Identifier: "ABC-3", Title: "Third", State: "Todo"}

	issueByID := map[string]linearapi.Issue{
		issue1.ID: issue1,
		issue2.ID: issue2,
		issue3.ID: issue3,
	}
	app.fetchIssueByID = func(ctx context.Context, id string) (linearapi.Issue, error) {
		return issueByID[id], nil
	}

	var mode atomic.Int32
	blockNext := make(chan struct{})
	app.fetchIssuesPage = func(ctx context.Context, params linearapi.FetchIssuesParams, after *string) (linearapi.IssuePage, error) {
		if mode.Load() == 0 {
			if after == nil {
				return linearapi.IssuePage{
					Issues:    []linearapi.Issue{issue1},
					HasNext:   true,
					EndCursor: stringPtr("cursor-1"),
				}, nil
			}
			<-blockNext
			return linearapi.IssuePage{
				Issues:  []linearapi.Issue{issue2},
				HasNext: false,
			}, nil
		}

		if after == nil {
			return linearapi.IssuePage{
				Issues:  []linearapi.Issue{issue3},
				HasNext: false,
			}, nil
		}

		return linearapi.IssuePage{}, nil
	}

	app.refreshIssues()
	waitForCondition(t, time.Second, func() bool {
		app.issuesMu.RLock()
		defer app.issuesMu.RUnlock()
		return len(app.issues) == 1
	})

	mode.Store(1)
	app.refreshIssues()
	close(blockNext)

	waitForCondition(t, time.Second, func() bool {
		app.issuesMu.RLock()
		defer app.issuesMu.RUnlock()
		return len(app.issues) == 1 && app.issues[0].ID == issue3.ID
	})
	app.issuesMu.RLock()
	issueID := app.issues[0].ID
	app.issuesMu.RUnlock()
	if issueID == issue2.ID {
		t.Fatalf("stale issue applied, got %s", issueID)
	}
}

func TestRefreshIssues_PreservesNavigationFocus(t *testing.T) {
	cfg := config.Config{
		PageSize: 1,
		CacheTTL: time.Minute,
	}
	app := NewApp(&linearapi.Client{}, cfg, nil)
	app.queueUpdateDraw = func(f func()) { f() }

	issue := linearapi.Issue{ID: "issue-1", Identifier: "ABC-1", Title: "First", State: "Todo"}
	app.fetchIssueByID = func(ctx context.Context, id string) (linearapi.Issue, error) {
		return issue, nil
	}
	app.fetchIssuesPage = func(ctx context.Context, params linearapi.FetchIssuesParams, after *string) (linearapi.IssuePage, error) {
		return linearapi.IssuePage{
			Issues:  []linearapi.Issue{issue},
			HasNext: false,
		}, nil
	}

	app.focusedPane = FocusNavigation
	app.refreshIssuesWithFocusChange(false)

	waitForCondition(t, time.Second, func() bool {
		app.issuesMu.RLock()
		defer app.issuesMu.RUnlock()
		return len(app.issues) == 1
	})

	if app.focusedPane != FocusNavigation {
		t.Fatalf("focusedPane = %v, want %v", app.focusedPane, FocusNavigation)
	}
}

func TestRefreshIssues_IncludesStateID(t *testing.T) {
	cfg := config.Config{
		PageSize: 1,
		CacheTTL: time.Minute,
	}
	app := NewApp(&linearapi.Client{}, cfg, nil)
	app.queueUpdateDraw = func(f func()) { f() }

	called := make(chan linearapi.FetchIssuesParams, 1)
	app.fetchIssuesPage = func(ctx context.Context, params linearapi.FetchIssuesParams, after *string) (linearapi.IssuePage, error) {
		select {
		case called <- params:
		default:
		}
		return linearapi.IssuePage{Issues: []linearapi.Issue{}, HasNext: false}, nil
	}

	app.selectedNavigation = &NavigationNode{
		ID:        "state-123",
		Text:      "In Progress",
		TeamID:    "team-1",
		IsStatus:  true,
		StateID:   "state-123",
		StateName: "In Progress",
	}

	app.refreshIssues()

	select {
	case params := <-called:
		if params.StateID != "state-123" {
			t.Fatalf("StateID = %q, want %q", params.StateID, "state-123")
		}
		if params.TeamID != "team-1" {
			t.Fatalf("TeamID = %q, want %q", params.TeamID, "team-1")
		}
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for fetchIssuesPage")
	}
}
