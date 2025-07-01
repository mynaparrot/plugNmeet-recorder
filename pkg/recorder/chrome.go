package recorder

import (
	"context"
	"errors"
	"fmt"
	"github.com/chromedp/cdproto/target"
	"github.com/chromedp/chromedp"
	log "github.com/sirupsen/logrus"
	"path"
	"time"
)

// launch Chrome to access URL
func (r *Recorder) launchChrome() {
	log.Infoln(fmt.Sprintf("launching chrome for task: %s, with url:%s", r.Req.Task.String(), r.joinUrl))

	opts := []chromedp.ExecAllocatorOption{
		// ---- Performance & Stability Flags ----
		chromedp.DisableGPU,
		chromedp.Flag("disable-background-networking", true),
		chromedp.Flag("disable-background-timer-throttling", true),
		chromedp.Flag("disable-backgrounding-occluded-windows", true),
		chromedp.Flag("disable-breakpad", true),
		chromedp.Flag("disable-client-side-phishing-detection", true),
		chromedp.Flag("disable-dev-shm-usage", true), // Crucial for Docker/containerized environments
		chromedp.Flag("disable-extensions", true),
		chromedp.Flag("disable-features", "site-per-process,Translate,TranslateUI"),
		chromedp.Flag("disable-hang-monitor", true),
		chromedp.Flag("disable-ipc-flooding-protection", true),
		chromedp.Flag("disable-renderer-backgrounding", true),
		chromedp.Flag("disable-sync", true),
		chromedp.Flag("metrics-recording-only", true),
		chromedp.Flag("safebrowsing-disable-auto-update", true),

		// ---- Automation & UI Control Flags ----
		chromedp.NoFirstRun,
		chromedp.NoDefaultBrowserCheck,
		chromedp.Flag("excludeSwitches", "enable-automation"), // Removes the "controlled by automation" bar
		chromedp.Flag("disable-default-apps", true),
		chromedp.Flag("disable-popup-blocking", true),
		chromedp.Flag("disable-prompt-on-repost", true),
		chromedp.Flag("password-store", "basic"), // Prevents prompts for OS-level keyrings
		chromedp.Flag("use-mock-keychain", true),
		chromedp.Flag("kiosk", true),
		chromedp.Flag("disable-notifications", true),
		chromedp.Flag("autoplay-policy", "no-user-gesture-required"),
		chromedp.Flag("window-position", "0,0"),
		chromedp.Flag("window-size", fmt.Sprintf("%d,%d", r.AppCnf.Recorder.Width, r.AppCnf.Recorder.Height)),
		chromedp.Flag("force-device-scale-factor", "1"),

		// ---- Environment & Rendering Flags ----
		chromedp.NoSandbox,
		chromedp.Flag("force-color-profile", "srgb"),
		chromedp.Env(fmt.Sprintf("PULSE_SINK=%s", r.pulseSinkName)),
		chromedp.Flag("display", r.displayId),
	}

	if r.AppCnf.Recorder.CustomChromePath != nil && *r.AppCnf.Recorder.CustomChromePath != "" {
		opts = append(opts, chromedp.ExecPath(*r.AppCnf.Recorder.CustomChromePath))
	}

	allocCtx, allocCancel := chromedp.NewExecAllocator(r.ctx, opts...)
	chromeCtx, chromeCancel := chromedp.NewContext(allocCtx)
	r.Lock()
	r.closeChrome = func() {
		chromeCancel()
		allocCancel()
	}
	r.Unlock()

	chromedp.ListenBrowser(chromeCtx, func(ev interface{}) {
		switch ev.(type) {
		case *target.EventDetachedFromTarget:
			log.Infoln(fmt.Sprintf("browser detached from target for task: %s, roomTableId: %d", r.Req.Task.String(), r.Req.GetRoomTableId()))
			r.Close(errors.New("browser detached from target unexpectedly"))
		case *target.EventTargetCrashed:
			log.Infoln(fmt.Sprintf("browser crashed for task: %s, roomTableId: %d", r.Req.Task.String(), r.Req.GetRoomTableId()))
			r.Close(errors.New("browser crashed"))
		}
	})

	err := chromedp.Run(chromeCtx,
		chromedp.Navigate(r.joinUrl),
		r.waitVisibleWithTimeout("div[id=startupJoinModal]", waitForSelectorTimeout),
		chromedp.Click("button[id=listenOnlyJoin]", chromedp.NodeVisible),
		r.waitVisibleWithTimeout("div[id=main-area]", waitForSelectorTimeout),
		chromedp.ActionFunc(func(context.Context) error {
			// wait to make sure videos are all loaded properly
			time.Sleep(time.Second * 3)
			return r.launchFfmpegProcess(path.Join(r.filePath, r.fileName))
		}),
		chromedp.WaitVisible("div[id=errorPage]"),
		chromedp.ActionFunc(func(context.Context) error {
			log.Infoln("got closing tag, so closing recorder now")
			r.Close(nil)
			return nil
		}),
	)
	if err != nil {
		if !errors.Is(err, context.Canceled) {
			log.Errorln("chrome:", err)
		}
		r.Close(err)
	}
}

func (r *Recorder) closeChromeDp() {
	r.Lock()
	defer r.Unlock()

	if r.closeChrome != nil {
		log.Infoln(fmt.Sprintf("closing chrome for task: %s, roomTableId: %d", r.Req.Task.String(), r.Req.GetRoomTableId()))

		r.closeChrome()
		r.closeChrome = nil
	}
}

func (r *Recorder) waitVisibleWithTimeout(selector string, timeout time.Duration) chromedp.ActionFunc {
	return func(ctx context.Context) error {
		timeoutCtx, cancel := context.WithTimeout(ctx, timeout)
		defer cancel()
		err := chromedp.WaitVisible(selector).Do(timeoutCtx)
		if err != nil && errors.Is(err, context.DeadlineExceeded) {
			err = errors.New(fmt.Sprintf("%s was not visible after %v", selector, timeout))
		}
		return err
	}
}
