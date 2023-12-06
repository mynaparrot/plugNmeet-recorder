// eslint-disable-next-line @typescript-eslint/no-unused-vars
/* global chrome, navigator, MediaRecorder, FileReader */
let recordingTabId = null;

const prepareTab = async (msg) => {
  const tabs = await chrome.tabs.query({
    active: true,
    lastFocusedWindow: true,
    currentWindow: true,
  });
  const currentTab = tabs[0];

  const tab = await chrome.tabs.create({
    url: chrome.runtime.getURL('pnm_recorder.html'),
    pinned: true,
    active: false,
  });

  chrome.tabs.onUpdated.addListener(async function listener(tabId, info) {
    if (tabId === tab.id && info.status === 'complete') {
      chrome.tabs.onUpdated.removeListener(listener);

      recordingTabId = tabId;
      await chrome.tabs.sendMessage(tabId, {
        type: msg.type,
        data: msg.data,
        currentTabId: currentTab.id,
      });
    }
  });
};

chrome.runtime.onConnect.addListener((port) => {
  port.onMessage.addListener(async (msg) => {
    if (msg.type === 'REC_CLIENT_PLAY') {
      await prepareTab(msg);
    } else {
      if (recordingTabId) {
        await chrome.tabs.sendMessage(recordingTabId, msg);
      }
    }
  });
});
