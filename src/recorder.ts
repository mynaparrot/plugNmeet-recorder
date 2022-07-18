import puppeteer, {
  BrowserConnectOptions,
  BrowserLaunchArgumentOptions,
  LaunchOptions,
} from 'puppeteer';
import os from 'os';
// eslint-disable-next-line @typescript-eslint/ban-ts-comment
// @ts-ignore
import Xvfb from 'xvfb';

import {
  FromChildToParent,
  FromParentToChild,
  RecorderArgs,
} from './utils/interfaces';
import { logger, sleep } from './utils/helper';

const args: any = process.argv.slice(2),
  platform = os.platform();

let browser: puppeteer.Browser, page: puppeteer.Page, xvfb: any;
let hasError = false,
  errorMessage: any,
  closedBycmd = false,
  wasCalledClose = false;

if (!args) {
  logger.error('no args found, closing..');
  process.exit();
}

const width = 1800;
const height = 900;

if (platform === 'linux') {
  // prettier-ignore
  xvfb = new Xvfb({
    silent: true,
    xvfb_args: [
      '-screen', '0', `${width}x${height}x24`, '-ac', '-nolisten', 'tcp', '-dpi', '200', '+extension', 'RANDR',
    ],
  });
}

const recorderArgs: RecorderArgs = JSON.parse(args);

const closeConnection = async (hasError: boolean, msg: string) => {
  let task = 'recording-ended';
  if (recorderArgs.serviceType === 'rtmp') {
    task = 'rtmp-ended';
  }

  const toParent: FromChildToParent = {
    status: true,
    task,
    msg,
    sid: recorderArgs.sid,
    room_id: recorderArgs.room_id,
    record_id: recorderArgs.record_id,
  };
  // eslint-disable-next-line @typescript-eslint/ban-ts-comment
  // @ts-ignore
  process.send(toParent);
};

const recordingStartedMsg = async (msg: string) => {
  let task = 'recording-started';
  if (recorderArgs.serviceType === 'rtmp') {
    task = 'rtmp-started';
  }

  const toParent: FromChildToParent = {
    status: true,
    task,
    msg,
    sid: recorderArgs.sid,
    room_id: recorderArgs.room_id,
    record_id: recorderArgs.record_id,
  };
  // eslint-disable-next-line @typescript-eslint/ban-ts-comment
  // @ts-ignore
  process.send(toParent);
};

const stopRecorder = async () => {
  try {
    await page.evaluate(() => {
      window.postMessage({ type: 'REC_STOP' }, '*');
    });
    await sleep(1000);
    await page.close();
  } catch (e) {
    logger.error('Error during stopRecorder');
  }

  return;
};

const onCloseOrErrorEvent = async () => {
  // prevent from multiple call
  if (wasCalledClose) {
    return;
  }
  wasCalledClose = true;

  if (closedBycmd) {
    await closeConnection(false, '');
  } else {
    await closeConnection(hasError, errorMessage);
  }

  // clear everything else
  await closeBrowser();

  // wait few moments
  await sleep(1500);
  process.exit();
};

// this method should call to clean all browser instances
const closeBrowser = async () => {
  try {
    await browser.close();
  } catch (e) {
    logger.error('Error during closeBrowser');
  }

  if (platform === 'linux') {
    try {
      xvfb.stopSync();
    } catch (e) {
      logger.error('Error during stop xvfb');
    }

    // sometime xvfb requite time to close
    await sleep(1000);
  }

  return;
};

process.on('SIGINT', async () => {
  logger.info('Child: got SIGINT, cleaning up');
  if (!wasCalledClose) {
    await onCloseOrErrorEvent();
  } else {
    await closeBrowser();
    process.exit();
  }
});

process.on('message', async (msg: FromParentToChild) => {
  if (msg.task === 'stop-recording' || msg.task === 'stop-rtmp') {
    logger.info('Child: ' + msg.task + ' sid: ' + msg.sid);
    closedBycmd = true;
    await stopRecorder();
  }
});

const options:
  | LaunchOptions
  | BrowserLaunchArgumentOptions
  | BrowserConnectOptions = {
  headless: false,
  args: [
    '--enable-usermedia-screen-capturing',
    '--allow-http-screen-capture',
    '--auto-select-desktop-capture-source=recorder',
    '--load-extension=' + __dirname + '/chrome-extension',
    '--disable-extensions-except=' + __dirname + '/chrome-extension',
    '--disable-infobars',
    '--shm-size=1gb',
    '--disable-dev-shm-usage',
    '--no-sandbox',
    '--no-zygote',
    '--start-fullscreen',
    '--app=https://www.google.com/',
    `--window-size=${width},${height}`,
  ],
  executablePath: '/usr/bin/google-chrome',
  defaultViewport: null,
};

if (platform == 'darwin') {
  (options as any).executablePath =
    '/Applications/Google Chrome.app/Contents/MacOS/Google Chrome';
}
// override with custom_chrome_path
if (recorderArgs.custom_chrome_path) {
  (options as any).executablePath = recorderArgs.custom_chrome_path;
}

(async () => {
  let url;
  if (recorderArgs.plugNmeetInfo.join_host) {
    url = recorderArgs.plugNmeetInfo.join_host + recorderArgs.access_token;
  } else {
    url =
      recorderArgs.plugNmeetInfo.host +
      '/?access_token=' +
      recorderArgs.access_token;
  }

  try {
    if (platform == 'linux') {
      try {
        xvfb.startSync();
      } catch (e: any) {
        await closeConnection(true, e.message);
        process.exit(1);
      }
      await sleep(1000);
    }

    browser = await puppeteer.launch(options);
    browser.on('disconnected', () => {
      logger.info('browser on disconnected');
      // this is just for safety
      // in any case it wasn't run page close event
      onCloseOrErrorEvent();
    });

    const pages = await browser.pages();
    page = pages[0];

    page.on('close', () => {
      logger.info('page on close');
      // this should call first
      onCloseOrErrorEvent();
    });

    page.on('error', async (e) => {
      logger.error('page on error');
      hasError = true;
      errorMessage = e.message;

      onCloseOrErrorEvent();
    });

    await page.exposeFunction('onMessageReceivedEvent', (e: any) => {
      if (e.data.type === 'WEBSOCKET_ERROR') {
        logger.error('on WEBSOCKET_ERROR');
        // we'll stop recorder
        hasError = true;
        errorMessage = 'WEBSOCKET_ERROR';
        onCloseOrErrorEvent();
      }
    });
    // eslint-disable-next-line @typescript-eslint/ban-ts-comment
    // @ts-ignore
    function listenFor(type) {
      return page.evaluateOnNewDocument((type: any) => {
        window.addEventListener(type, (e) => {
          (window as any).onMessageReceivedEvent({ type, data: e.data });
        });
      }, type);
    }
    // we'll listen websocket message
    await listenFor('message');

    await page.goto(url, {
      waitUntil: 'networkidle2',
    });

    await page.waitForSelector('div[id=startupJoinModal]', {
      timeout: 20 * 1000,
    });
    await page.click('button[id=listenOnlyJoin]');
    await page.waitForSelector('div[id=main-area]', { timeout: 20 * 1000 });

    await page.evaluate((websocket_url) => {
      window.postMessage(
        {
          type: 'START_WEBSOCKET',
          websocket_url,
        },
        '*',
      );
    }, recorderArgs.websocket_url);

    // we should notify
    await recordingStartedMsg('started');
    await page.evaluate(() => {
      window.postMessage({ type: 'REC_START' }, '*');
    });

    await page.waitForSelector('div[id=errorPage]', { timeout: 0 });
    await stopRecorder();
  } catch (e: any) {
    hasError = true;
    errorMessage = e.message;
    if (!closedBycmd) {
      logger.error(e.message);
    }
  } finally {
    // we'll close everything
    onCloseOrErrorEvent();
  }
})();
