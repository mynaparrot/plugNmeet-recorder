// eslint-disable-next-line @typescript-eslint/no-unused-vars
/* global chrome, navigator, MediaRecorder, FileReader */

let recorder = null;
let ws;
let closeByCmd = false;

chrome.runtime.onConnect.addListener((port) => {
  port.onMessage.addListener((msg) => {
    switch (msg.type) {
      case 'START_WEBSOCKET':
        startWebsocket(msg.websocket_url, port);
        break;

      case 'REC_STOP':
        closeByCmd = true;
        recorder.stop();
        break;

      case 'REC_START':
        recorder.start(1000);
        break;

      case 'REC_CLIENT_PLAY':
        if (recorder) {
          break;
        }
        startScreenSharing(msg, port);
        break;
      default:
        console.log('Unrecognized message', msg);
    }
  });
});

function startScreenSharing(msg, port) {
  const { tab } = port.sender;
  tab.url = msg.data.url;
  chrome.desktopCapture.chooseDesktopMedia(['tab', 'audio'], (streamId) => {
    // Get the stream
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
      },
      (stream) => {
        recorder = new MediaRecorder(stream, {
          width: { min: 1280, ideal: 1920, max: 1920 },
          height: { min: 720, ideal: 1080, max: 1080 },
          frameRate: { min: 30, ideal: 30 },
          videoBitsPerSecond: 3000000,
          audioBitsPerSecond: 128000,
          ignoreMutedMedia: true,
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
}

function startWebsocket(url, port) {
  ws = new WebSocket(url);

  ws.addEventListener('error', () => {
    if (!closeByCmd) {
      port.postMessage({ websocketError: ws.readyState });
    }
  });
  ws.addEventListener('close', () => {
    if (!closeByCmd) {
      port.postMessage({ websocketError: ws.readyState });
    }
  });
}
