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
		chromedp.NoFirstRun,
		chromedp.NoDefaultBrowserCheck,
		chromedp.DisableGPU,
		chromedp.NoSandbox,

		// puppeteer default behavior
		chromedp.Flag("disable-infobars", true),
		chromedp.Flag("excludeSwitches", "enable-automation"),
		chromedp.Flag("disable-background-networking", true),
		chromedp.Flag("enable-features", "NetworkService,NetworkServiceInProcess"),
		chromedp.Flag("disable-background-timer-throttling", true),
		chromedp.Flag("disable-backgrounding-occluded-windows", true),
		chromedp.Flag("disable-breakpad", true),
		chromedp.Flag("disable-client-side-phishing-detection", true),
		chromedp.Flag("disable-default-apps", true),
		chromedp.Flag("disable-dev-shm-usage", true),
		chromedp.Flag("disable-extensions", true),
		chromedp.Flag("disable-features", "AudioServiceOutOfProcess,site-per-process,Translate,TranslateUI,BlinkGenPropertyTrees"),
		chromedp.Flag("disable-hang-monitor", true),
		chromedp.Flag("disable-ipc-flooding-protection", true),
		chromedp.Flag("disable-popup-blocking", true),
		chromedp.Flag("disable-prompt-on-repost", true),
		chromedp.Flag("disable-renderer-backgrounding", true),
		chromedp.Flag("disable-sync", true),
		chromedp.Flag("force-color-profile", "srgb"),
		chromedp.Flag("metrics-recording-only", true),
		chromedp.Flag("safebrowsing-disable-auto-update", true),
		chromedp.Flag("password-store", "basic"),
		chromedp.Flag("use-mock-keychain", true),
		chromedp.Flag("disable-web-security", true),
		chromedp.Flag("allow-running-insecure-content", true),

		// custom args
		chromedp.Flag("kiosk", true),
		chromedp.Flag("disable-translate", true),
		chromedp.Flag("enable-automation", false),
		chromedp.Flag("disable-notifications", true),
		chromedp.Flag("autoplay-policy", "no-user-gesture-required"),
		chromedp.Flag("window-position", "0,0"),
		chromedp.Flag("window-size", fmt.Sprintf("%d,%d", r.AppCnf.Recorder.Width, r.AppCnf.Recorder.Height)),

		// output
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
			log.Infoln("browser detached from target")
			r.Close(errors.New("browser detached from target unexpectedly"))
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
	if r.closeChrome != nil {
		log.Infoln("closing chrome")
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
