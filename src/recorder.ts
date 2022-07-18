import puppeteer, {
  BrowserConnectOptions,
  BrowserLaunchArgumentOptions,
  LaunchOptions,
} from 'puppeteer';
import os from 'os';
import Redis, { RedisOptions } from 'ioredis';
// eslint-disable-next-line @typescript-eslint/ban-ts-comment
// @ts-ignore
import Xvfb from 'xvfb';

import {
  FromChildToParent,
  RecorderArgs,
  RecorderResp,
} from './utils/interfaces';
import { logger, notify, sleep } from './utils/helper';
import { updateRecorderProgress } from './utils/redisTasks';

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
const redisOptions: RedisOptions = {
  host: recorderArgs.redisInfo.host,
  port: recorderArgs.redisInfo.port,
  username: recorderArgs.redisInfo.username,
  password: recorderArgs.redisInfo.password,
  db: recorderArgs.redisInfo.db,
  connectionName: recorderArgs.recorder_id + '-fork',
};

let redis: Redis;

try {
  redis = new Redis(redisOptions);
} catch (e) {
  logger.error(e);
  process.exit(1);
}

const subNode = redis.duplicate();

subNode.subscribe('plug-n-meet-recorder', (err, count) => {
  if (err) {
    logger.error('Failed to subscribe: %s', err.message);
  } else {
    logger.info(
      `FORK recorder: Subscribed successfully! This client is currently subscribed to ${count} channels.`,
    );
  }
});

subNode.on('message', async (channel, message) => {
  const payload: RecorderResp = JSON.parse(message);

  if (
    payload.from === 'plugnmeet' &&
    ((payload.task === 'stop-recording' &&
      recorderArgs.serviceType === 'recording') ||
      (payload.task === 'stop-rtmp' && recorderArgs.serviceType === 'rtmp')) &&
    payload.sid === recorderArgs.sid
  ) {
    logger.info('FORK: ' + payload.task + ' sid: ' + payload.sid);
    closedBycmd = true;
    await stopRecorder();
  }
});

const closeConnection = async (hasError: boolean, msg: string) => {
  let task = 'recording-ended';
  if (recorderArgs.serviceType === 'rtmp') {
    task = 'rtmp-ended';
  }

  const payload: RecorderResp = {
    from: 'recorder',
    status: !hasError,
    task,
    msg: msg,
    record_id: recorderArgs.record_id,
    sid: recorderArgs.sid,
    room_id: recorderArgs.room_id,
    recorder_id: recorderArgs.recorder_id, // this recorder ID
  };

  await notify(recorderArgs.plugNmeetInfo, payload);
  await updateRecorderProgress(redis, recorderArgs.recorder_id, false);

  // wait few moments
  await sleep(1500);
  await redis.quit();
  await subNode.quit();
  process.exit();
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

process.on('disconnect', async () => {
  await onCloseOrErrorEvent();
  process.exit();
});
process.on('message', (e) => {
  console.log(e);
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
    //'--start-fullscreen',
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
