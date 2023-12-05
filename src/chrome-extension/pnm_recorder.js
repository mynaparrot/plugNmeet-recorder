// eslint-disable-next-line @typescript-eslint/no-unused-vars
/* global chrome */

let recorder = null,
  ws,
  closeByCmd = false,
  currentTab = null;

const prepareRecorder = async (url) => {
  const tabs = await chrome.tabs.query({});

  for (let i = 0; i < tabs.length; i++) {
    const t = tabs[i];
    if (t.url === url) {
      currentTab = t;
    }
  }

  await chrome.tabs.update(currentTab.id, {
    active: true,
    highlighted: true,
  });

  chrome.desktopCapture.chooseDesktopMedia(['tab', 'audio'], (streamId) => {
    navigator.webkitGetUserMedia(
      {
        audio: {
          mandatory: {
            chromeMediaSource: 'system',
          },
        },
        video: {
          mandatory: {
            chromeMediaSource: 'desktop',
            chromeMediaSourceId: streamId,
            minWidth: 1920,
            maxWidth: 1920,
            minHeight: 1080,
            maxHeight: 1080,
            minFrameRate: 60,
          },
        },
        preferCurrentTab: true,
      },
      (stream) => {
        recorder = new MediaRecorder(stream, {
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
            if (ws.readyState === WebSocket.OPEN) {
              ws.send(event.data);
            }
          }
        };

        recorder.onstop = () => {
          ws.close();
        };
      },
      (error) => console.log('Unable to get user media', error),
    );
  });
};

const startWebsocket = (url) => {
  ws = new WebSocket(url);

  ws.addEventListener('error', () => {
    if (!closeByCmd) {
      //  send messages to content_script.js will require using tabs
      chrome.tabs.sendMessage(currentTab.id, { websocketError: ws.readyState });
    }
  });
  ws.addEventListener('close', () => {
    if (!closeByCmd) {
      chrome.tabs.sendMessage(currentTab.id, { websocketError: ws.readyState });
    }
  });
};

chrome.runtime.onMessage.addListener((msg) => {
  console.log(msg.type);
  switch (msg.type) {
    case 'START_WEBSOCKET':
      startWebsocket(msg.websocket_url);
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
      prepareRecorder(msg.data.url);
      break;
    default:
      console.log('Unrecognized message', msg);
  }
});
