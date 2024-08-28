import puppeteer, {
  BrowserConnectOptions,
  BrowserLaunchArgumentOptions,
  LaunchOptions,
  Browser,
  Page,
} from 'puppeteer';
import os from 'os';
// eslint-disable-next-line @typescript-eslint/ban-ts-comment
// @ts-ignore
import Xvfb from 'xvfb';
import { create, fromJsonString, toJsonString } from '@bufbuild/protobuf';

import { logger, sleep } from './utils/helper';
import {
  FromChildToParentSchema,
  FromParentToChildSchema,
  RecorderServiceType,
  RecordingTasks,
  StartRecorderChildArgsSchema,
} from './proto/plugnmeet_recorder_pb';

const args = process.argv.slice(2),
  platform = os.platform();

let browser: Browser, page: Page, xvfb: any;
let hasError = false,
  errorMessage: any,
  closedBycmd = false,
  wasCalledClose = false;

if (!args.length) {
  logger.error('no args found, closing..');
  process.exit();
}
const recorderArgs = fromJsonString(StartRecorderChildArgsSchema, args[0]);

const width = recorderArgs.width || 1800;
const height = recorderArgs.height || 900;
const dpi = recorderArgs.xvfbDpi || 200;
if (platform === 'linux') {
  // prettier-ignore
  xvfb = new Xvfb({
    silent: true,
    xvfb_args: [
      '-screen', '0', `${width}x${height}x24`, '-ac', '-nolisten', 'tcp', '-dpi', `${dpi}`, '+extension', 'RANDR',
    ],
  });
}

const closeConnection = async (hasError: boolean, msg: string) => {
  let task = RecordingTasks.END_RECORDING;
  if (recorderArgs.serviceType === RecorderServiceType.RTMP) {
    task = RecordingTasks.END_RTMP;
  }

  const toParent = create(FromChildToParentSchema, {
    status: true,
    task,
    msg,
    roomTableId: recorderArgs.roomTableId,
    recordingId: recorderArgs.recordingId,
  });

  // eslint-disable-next-line @typescript-eslint/ban-ts-comment
  // @ts-expect-error
  process.send(toJsonString(FromChildToParentSchema, toParent));
};

const recordingStartedMsg = async (msg: string) => {
  let task = RecordingTasks.START_RECORDING;
  if (recorderArgs.serviceType === RecorderServiceType.RTMP) {
    task = RecordingTasks.START_RTMP;
  }

  const toParent = create(FromChildToParentSchema, {
    status: true,
    task,
    msg,
    roomTableId: recorderArgs.roomTableId,
    recordingId: recorderArgs.recordingId,
  });
  // eslint-disable-next-line @typescript-eslint/ban-ts-comment
  // @ts-ignore
  process.send(toJsonString(FromChildToParentSchema, toParent));
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

  // wait a few moments
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

process.on('message', async (m: string) => {
  const msg = fromJsonString(FromParentToChildSchema, m);
  if (
    msg.task === RecordingTasks.STOP_RECORDING ||
    msg.task === RecordingTasks.STOP_RTMP
  ) {
    logger.info('Child: ' + msg.task + ' sid: ' + msg.roomTableId);
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
  defaultViewport: null,
  protocolTimeout: 0,
};

// override with custom_chrome_path
if (recorderArgs.customChromePath) {
  (options as any).executablePath = recorderArgs.customChromePath;
}

(async () => {
  let url;
  if (recorderArgs.plugNMeetInfo?.joinHost) {
    url = recorderArgs.plugNMeetInfo.joinHost + recorderArgs.accessToken;
  } else {
    url =
      recorderArgs.plugNMeetInfo?.host +
      '/?access_token=' +
      recorderArgs.accessToken;
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
      // this is just for safety,
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
    // we'll listen to the websocket message
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
    }, recorderArgs.websocketUrl);

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
    await onCloseOrErrorEvent();
  }
})();
