package recorder

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/chromedp/cdproto/target"
	"github.com/chromedp/chromedp"
	"github.com/mynaparrot/plugnmeet-protocol/plugnmeet"
	"github.com/sirupsen/logrus"
)

// launch Chrome to access URL
func (r *Recorder) launchChrome() {
	log := r.Logger.WithField("joinUrl", r.joinUrl)
	log.Infof("Launching chrome")

	// Default Chrome flags
	flags := map[string]interface{}{
		// ---- Performance & Stability Flags ----
		"disable-gpu":                            true, // DisableGPU
		"disable-background-networking":          true,
		"disable-background-timer-throttling":    true,
		"disable-backgrounding-occluded-windows": true,
		"disable-breakpad":                       true,
		"disable-client-side-phishing-detection": true,
		"disable-dev-shm-usage":                  true, // Crucial for Docker/containerized environments
		"disable-extensions":                     true,
		"disable-features":                       "site-per-process,Translate,TranslateUI",
		"disable-hang-monitor":                   true,
		"disable-ipc-flooding-protection":        true,
		"disable-renderer-backgrounding":         true,
		"disable-sync":                           true,
		"metrics-recording-only":                 true,
		"safebrowsing-disable-auto-update":       true,

		// ---- Automation & UI Control Flags ----
		"no-first-run":              true, // NoFirstRun
		"no-default-browser-check":  true, // NoDefaultBrowserCheck
		"excludeSwitches":           "enable-automation",
		"disable-default-apps":      true,
		"disable-popup-blocking":    true,
		"disable-prompt-on-repost":  true,
		"password-store":            "basic",
		"use-mock-keychain":         true,
		"kiosk":                     true,
		"disable-notifications":     true,
		"autoplay-policy":           "no-user-gesture-required",
		"window-position":           "0,0",
		"force-device-scale-factor": "1",

		// ---- Environment & Rendering Flags ----
		"no-sandbox":          true, // NoSandbox
		"force-color-profile": "srgb",
	}

	// Merge user's custom flags (replaces defaults with same key)
	if r.AppCnf.ChromeSettings != nil {
		for _, cf := range r.AppCnf.ChromeSettings.Flags {
			flags[cf.Name] = cf.Value
		}
	}

	// Build opts from map
	var opts []chromedp.ExecAllocatorOption
	for name, value := range flags {
		opts = append(opts, chromedp.Flag(name, value))
	}

	// Non-editable / required options
	opts = append(opts,
		chromedp.Flag("window-size", fmt.Sprintf("%d,%d", r.AppCnf.Recorder.Width, r.AppCnf.Recorder.Height)),
		chromedp.Env(fmt.Sprintf("PULSE_SINK=%s", r.pulseSinkName)),
		chromedp.Flag("display", r.displayId),
	)

	if r.AppCnf.ChromeSettings != nil && r.AppCnf.ChromeSettings.CustomChromePath != nil && *r.AppCnf.ChromeSettings.CustomChromePath != "" {
		opts = append(opts, chromedp.ExecPath(*r.AppCnf.ChromeSettings.CustomChromePath))
	}

	allocCtx, allocCancel := chromedp.NewExecAllocator(r.ctx, opts...)
	chromeCtx, chromeCancel := chromedp.NewContext(allocCtx)
	r.Lock()
	r.closeChrome = func() {
		chromeCancel()
		allocCancel()
	}
	r.Unlock()

	// Call BrowserEvents here, not inside the goroutines: it starts the browser
	// on first use, and that start is not concurrency-safe.
	detached := chromedp.BrowserEvents(chromeCtx, target.DetachedFromTarget)
	crashed := chromedp.BrowserEvents(chromeCtx, target.TargetCrashed)

	go func() {
		for _, err := range detached {
			if err != nil {
				return
			}
			if chromeCtx.Err() != nil {
				return
			}
			log.Errorln("Browser detached from target unexpectedly")
			r.Close(plugnmeet.RecordingTasks_STOP, errors.New("browser detached from target unexpectedly"))
			return
		}
	}()
	go func() {
		for _, err := range crashed {
			if err != nil {
				fmt.Println(err)
				return
			}
			if chromeCtx.Err() != nil {
				return
			}
			log.Errorln("Browser crashed")
			r.Close(plugnmeet.RecordingTasks_STOP, errors.New("browser crashed"))
			return
		}
	}()

	err := chromedp.Do(chromeCtx,
		chromedp.Navigate(r.joinUrl),
		r.waitVisibleWithTimeout("div[id=startupJoinModal]", waitForSelectorTimeout),
		chromedp.Click("button[id=listenOnlyJoin]", chromedp.NodeVisible),
		// Move the mouse to the top-left corner to remove hover effects.
		chromedp.MouseEvent(chromedp.MouseMoved, 0, 0),
		r.waitVisibleWithTimeout("div[id=main-area]", waitForSelectorTimeout),
		chromedp.Func(func(context.Context, *chromedp.Target) error {
			// wait to make sure videos are all loaded properly
			time.Sleep(time.Second * 3)
			return r.launchFfmpegProcess()
		}),
		chromedp.WaitVisible("div[id=errorPage]"),
		chromedp.Func(func(context.Context, *chromedp.Target) error {
			log.Infoln("Got error page, closing recorder")
			r.Close(plugnmeet.RecordingTasks_STOP, nil)
			return nil
		}),
	)
	if err != nil {
		if !errors.Is(err, context.Canceled) {
			log.WithError(err).Error("Chromedp run error")
		}
		r.Close(plugnmeet.RecordingTasks_STOP, err)
	}
}

func (r *Recorder) closeChromeDp(log *logrus.Entry) {
	r.Lock()
	defer r.Unlock()

	if r.closeChrome != nil {
		log.Infoln("Closing chrome")

		r.closeChrome()
		r.closeChrome = nil
	}
}

func (r *Recorder) waitVisibleWithTimeout(selector string, timeout time.Duration) chromedp.Action[chromedp.Void] {
	return func(ctx context.Context, t *chromedp.Target) (chromedp.Void, error) {
		timeoutCtx, cancel := context.WithTimeout(ctx, timeout)
		defer cancel()
		_, err := chromedp.Run(timeoutCtx, chromedp.WaitVisible(selector))
		if err != nil && errors.Is(err, context.DeadlineExceeded) {
			err = fmt.Errorf("%s was not visible after %v", selector, timeout)
		}
		return chromedp.Void{}, err
	}
}
