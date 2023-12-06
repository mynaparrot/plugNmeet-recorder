// eslint-disable-next-line @typescript-eslint/no-unused-vars
/* global chrome */

let recorder = null,
  ws = null,
  closeByCmd = false,
  currentTab = null;

const prepareRecorder = async (url) => {
  const tabs = await chrome.tabs.query({});

  for (let i = 0; i < tabs.length; i++) {
    const t = tabs[i];
    if (t.url.search(url) > -1) {
      currentTab = t;
    }
  }

  await chrome.tabs.update(currentTab.id, {
    active: true,
    highlighted: true,
  });

  try {
    chrome.tabCapture.getMediaStreamId(
      { targetTabId: currentTab.id },
      async (id) => {
        const media = await navigator.mediaDevices.getUserMedia({
          audio: {
            mandatory: {
              chromeMediaSource: 'tab',
              chromeMediaSourceId: id,
            },
          },
          video: {
            mandatory: {
              chromeMediaSource: 'tab',
              chromeMediaSourceId: id,
              minWidth: 1920,
              maxWidth: 1920,
              minHeight: 1080,
              maxHeight: 1080,
              minFrameRate: 60,
            },
          },
          preferCurrentTab: true,
        });

        recorder = new MediaRecorder(media, {
          width: { min: 1280, ideal: 1920, max: 1920 },
          height: { min: 720, ideal: 1080, max: 1080 },
          frameRate: { min: 30, ideal: 30 },
          videoBitsPerSecond: 3000000,
          audioBitsPerSecond: 128000,
          ignoreMutedMedia: true,
          mimeType: 'video/webm;codecs=h264',
        });

        recorder.ondataavailable = (event) => {
          if (event.data.size > 0) {
            if (ws && ws.readyState === WebSocket.OPEN) {
              ws.send(event.data);
            }
          }
        };

        recorder.onstop = () => {
          ws.close();
        };
      },
    );
  } catch (error) {
    console.log('Unable to get user media', error);
    setTimeout(async () => {
      await chrome.tabs.sendMessage(currentTab.id, {
        tabCaptureError:
          'unable to start tabCapture: ' +
          error +
          '. ID: ' +
          chrome.runtime.id +
          ' tabId: ' +
          currentTab.id,
      });
    }, 1000);
  }
};

const startWebsocket = (url) => {
  ws = new WebSocket(url);

  ws.addEventListener('error', async () => {
    if (!closeByCmd) {
      //  send messages to content_script.js will require using tabs
      await chrome.tabs.sendMessage(currentTab.id, {
        websocketError: ws.readyState,
      });
    }
  });
  ws.addEventListener('close', async () => {
    if (!closeByCmd) {
      await chrome.tabs.sendMessage(currentTab.id, {
        websocketError: ws.readyState,
      });
    }
  });
};

chrome.runtime.onMessage.addListener(async (msg) => {
  switch (msg.type) {
    case 'START_WEBSOCKET':
      if (ws === null) {
        startWebsocket(msg.websocket_url);
      }
      break;

    case 'REC_STOP':
      closeByCmd = true;
      recorder.stop();
      break;

    case 'REC_START':
      if (recorder) {
        recorder.start(1000);
      }
      break;

    case 'REC_CLIENT_PLAY':
      if (recorder) {
        break;
      }
      await prepareRecorder(msg.data.url);
      break;
    default:
      console.log('Unrecognized message', msg);
  }
});
