package main

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/chromedp/cdproto/cdp"
	"github.com/chromedp/cdproto/page"
	"github.com/chromedp/chromedp"
	"github.com/chromedp/chromedp/remote"
)

const (
	debugPort   = "9333"
	debuggerURL = "http://127.0.0.1:" + debugPort
)

// openImpersonation opens the dashboard page that impersonates userID of env: in the dedicated
// Chrome, which clicks Impersonate, or in chrome when it is one of the user's own profiles.
func openImpersonation(ctx context.Context, env environment, userID string, chrome chromeProfile) error {
	query := url.Values{"user_id": {userID}, "auth_url": {env.AuthHostname}}
	page := "https://app.propelauth.com/impersonate-user?" + query.Encode()
	if chrome.dir != "" {
		return openInChromeProfile(ctx, chrome, page)
	}
	return impersonate(ctx, page)
}

// impersonate opens the dashboard impersonation page in the dedicated Chrome and confirms it.
// The tab stays open once the program detaches.
func impersonate(ctx context.Context, impersonateURL string) error {
	if err := ensureChrome(ctx); err != nil {
		return err
	}
	allocCtx, cancelAlloc := remote.NewAllocator(ctx, debuggerURL)
	defer cancelAlloc()
	tabCtx, cancelTab := chromedp.NewContext(allocCtx, chromedp.WithDetachOnCancel())
	defer cancelTab()
	// The first Run opens the tab: under a timeout, the deadline would close it.
	if err := chromedp.Do(tabCtx); err != nil {
		return fmt.Errorf("opening a tab: %w", err)
	}
	if _, err := chromedp.Call(tabCtx, page.BringToFront, cdp.Empty{}); err != nil {
		return err
	}

	// Long enough to sign in to the dashboard first, on a new profile or an expired session.
	clickCtx, cancelClick := context.WithTimeout(tabCtx, 3*time.Minute)
	defer cancelClick()
	err := chromedp.Do(clickCtx,
		chromedp.Navigate(impersonateURL),
		chromedp.Click(chromedp.Search(`//button[normalize-space()="Impersonate"]`)),
	)
	if err != nil {
		return fmt.Errorf("clicking Impersonate: %w", err)
	}
	return waitForRedirect(tabCtx)
}

// waitForRedirect waits for the dashboard to hand the impersonation token to the auth domain.
func waitForRedirect(ctx context.Context) error {
	ctx, cancel := context.WithTimeout(ctx, 20*time.Second)
	defer cancel()
	for {
		// Location fails while the page navigates: poll again.
		location, err := chromedp.Run(ctx, chromedp.Location())
		if err == nil && !strings.HasPrefix(location, "https://app.propelauth.com/") {
			return nil
		}
		select {
		case <-ctx.Done():
			return errors.New("the dashboard did not redirect after the click: check the Chrome window")
		case <-time.After(300 * time.Millisecond):
		}
	}
}

func ensureChrome(ctx context.Context) error {
	if debuggerUp(ctx) {
		return nil
	}
	configDir, err := os.UserConfigDir()
	if err != nil {
		return err
	}
	// A profile of its own: impersonating replaces the auth session of the browser it runs in,
	// and Chrome ignores the debugging port on its default profile.
	cmd := exec.CommandContext(ctx, "open", "-na", "Google Chrome", "--args",
		"--user-data-dir="+filepath.Join(configDir, "impersonate", "chrome"),
		"--remote-debugging-port="+debugPort,
		"--no-first-run",
		"--no-default-browser-check",
	)
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("launching Chrome: %w: %s", err, out)
	}
	for deadline := time.Now().Add(15 * time.Second); time.Now().Before(deadline); time.Sleep(200 * time.Millisecond) {
		if debuggerUp(ctx) {
			return nil
		}
	}
	return errors.New("Chrome did not open its debugging port " + debuggerURL)
}

func debuggerUp(ctx context.Context) bool {
	ctx, cancel := context.WithTimeout(ctx, 500*time.Millisecond)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, debuggerURL+"/json/version", nil)
	if err != nil {
		return false
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return false
	}
	resp.Body.Close()
	return resp.StatusCode == http.StatusOK
}
