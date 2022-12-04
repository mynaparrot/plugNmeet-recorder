import Redis from 'ioredis';
import yaml from 'js-yaml';
import * as fs from 'fs';
import { fork } from 'child_process';

import {
  ChildProcessInfoMap,
  PlugNmeetInfo,
  Recorder,
  RedisInfo,
  WebsocketServerInfo,
} from './utils/interfaces';
import { logger, notify, sleep } from './utils/helper';
import {
  addRecorder,
  openRedisConnection,
  sendPing,
  updateRecorderProgress,
} from './utils/redisTasks';
import { ChildProcess } from 'concurrently/dist/src/command';
import {
  FromChildToParent,
  FromParentToChild,
  PlugNmeetToRecorder,
  RecorderServiceType,
  RecorderToPlugNmeet,
  RecordingTasks,
  StartRecorderChildArgs,
} from './proto/plugnmeet_recorder_pb';

let redisInfo: RedisInfo;
let recorder: Recorder;
let plugNmeetInfo: PlugNmeetInfo;
let websocketServerInfo: WebsocketServerInfo;
let redis: Redis, subNode: Redis;
const childProcessesInfoMapByChildPid = new Map<number, ChildProcessInfoMap>();
const childProcessesMapByRoomSid = new Map<string, any>();

try {
  const config: any = yaml.load(fs.readFileSync('config.yaml', 'utf8'));
  redisInfo = config.redis_info;
  recorder = config.recorder;
  plugNmeetInfo = config.plugNmeet_info;
  websocketServerInfo = config.websocket_server;
} catch (e) {
  logger.error('Error: ', e);
  process.exit();
}

process.on('SIGINT', async () => {
  logger.info('Caught interrupt signal, cleaning up');

  childProcessesMapByRoomSid.forEach((c) => c.kill('SIGINT'));
  await sleep(5000);

  if (redis && redis.status === 'connect') {
    redis.disconnect();
    subNode.disconnect();
  }
  // clear everything
  childProcessesMapByRoomSid.clear();
  childProcessesInfoMapByChildPid.clear();
  // now end the process
  process.exit();
});

(async () => {
  const redis = await openRedisConnection(redisInfo);
  if (!redis) {
    return;
  }
  const subNode = redis.duplicate();

  subNode.subscribe('plug-n-meet-recorder', async (err) => {
    if (err) {
      logger.error('Failed to subscribe: %s', err.message);
    } else {
      logger.info('Subscribed successfully! Waiting for message');
      await addRecorder(redis, recorder.id, recorder.max_limit);
      startPing();
    }
  });

  subNode.on('message', (channel, message) => {
    let payload: PlugNmeetToRecorder;
    try {
      payload = PlugNmeetToRecorder.fromJsonString(message);
    } catch (e) {
      logger.error(e);
      return;
    }
    if (payload.from !== 'plugnmeet') {
      return;
    }
    logger.info('Main: ' + payload.task);

    if (
      (payload.task === RecordingTasks.START_RECORDING ||
        payload.task === RecordingTasks.START_RTMP) &&
      payload.recorderId === recorder.id
    ) {
      handleStartRequest(payload);
    } else if (
      payload.task === RecordingTasks.STOP ||
      payload.task === RecordingTasks.STOP_RECORDING ||
      payload.task === RecordingTasks.STOP_RTMP
    ) {
      // for any stop task when meeting will end or have stop request
      if (
        childProcessesMapByRoomSid.has(
          RecorderServiceType.RECORDING + ':' + payload.roomSid,
        )
      ) {
        handleStopProcess(
          RecordingTasks.STOP_RECORDING,
          RecorderServiceType.RECORDING,
          payload.roomSid,
        );
      } else if (
        childProcessesMapByRoomSid.has(
          RecorderServiceType.RTMP + ':' + payload.roomSid,
        )
      ) {
        handleStopProcess(
          RecordingTasks.STOP_RTMP,
          RecorderServiceType.RTMP,
          payload.roomSid,
        );
      }
    }
  });

  const handleStopProcess = (
    task: RecordingTasks,
    serviceType: RecorderServiceType,
    roomSid: string,
  ) => {
    const child = childProcessesMapByRoomSid.get(serviceType + ':' + roomSid);
    if (child) {
      const recordInfo = childProcessesInfoMapByChildPid.get(child.pid);
      if (recordInfo) {
        const toChild = new FromParentToChild({
          task: task,
          recordingId: recordInfo.recording_id,
          roomId: recordInfo.room_id,
          roomSid: recordInfo.sid,
        });
        child?.send(toChild.toJsonString());
      }
    }
  };

  const handleStartRequest = (payload: PlugNmeetToRecorder) => {
    const websocket_url = `${websocketServerInfo.host}:${websocketServerInfo.port}?auth_token=${websocketServerInfo.auth_token}&room_id=${payload.roomId}&room_sid=${payload.roomSid}&recording_id=${payload.recordingId}`;

    const toSend = new StartRecorderChildArgs({
      roomId: payload.roomId,
      recordingId: payload.recordingId,
      roomSid: payload.roomSid,
      accessToken: payload.accessToken,
      plugNMeetInfo: {
        host: plugNmeetInfo.host,
        apiKey: plugNmeetInfo.api_key,
        apiSecret: plugNmeetInfo.api_secret,
        joinHost: plugNmeetInfo.join_host,
      },
      postMp4Convert: recorder.post_mp4_convert,
      copyToPath: {
        mainPath: recorder.copy_to_path.main_path,
        subPath: recorder.copy_to_path.sub_path,
      },
      recorderId: recorder.id,
      serviceType: RecorderServiceType.RECORDING,
      websocketUrl: websocket_url,
    });

    if (recorder.custom_chrome_path) {
      toSend.customChromePath = recorder.custom_chrome_path;
    }

    if (payload.task === RecordingTasks.START_RECORDING) {
      toSend.websocketUrl = toSend.websocketUrl + '&service=recording';
    } else if (payload.task === RecordingTasks.START_RTMP) {
      toSend.websocketUrl =
        toSend.websocketUrl + '&service=rtmp&rtmp_url=' + payload.rtmpUrl;
      toSend.serviceType = RecorderServiceType.RTMP;
    }

    let child: ChildProcess;

    if (typeof process.env.TS_NODE_DEV !== 'undefined') {
      child = fork('src/recorder', [toSend.toJsonString()], {
        execArgv: ['-r', 'ts-node/register'],
      });
    } else {
      child = fork('dist/recorder', [toSend.toJsonString()]);
    }

    if (child.pid) {
      const childProcessInfo: ChildProcessInfoMap = {
        serviceType: toSend.serviceType,
        recording_id: toSend.recordingId,
        room_id: toSend.roomId,
        sid: toSend.roomSid,
      };
      childProcessesInfoMapByChildPid.set(child.pid, childProcessInfo);
      childProcessesMapByRoomSid.set(
        toSend.serviceType + ':' + payload.roomSid,
        child,
      );
    }

    child.on('message', (msg: string) => {
      if (child.pid) {
        handleMsgFromChild(msg, child.pid);
      }
    });

    child.on('exit', (code: number) => {
      if (child.pid) {
        const recordInfo = childProcessesInfoMapByChildPid.get(child.pid);

        if (typeof recordInfo !== 'undefined') {
          // we can use same as FromChildToParent message format.
          const toChild = new FromChildToParent({
            msg: code === 0 ? 'no error' : 'had error',
            status: code === 0,
            task:
              recordInfo.serviceType === RecorderServiceType.RECORDING
                ? RecordingTasks.END_RECORDING
                : RecordingTasks.END_RTMP,
            recordingId: recordInfo.recording_id,
            roomId: recordInfo.room_id,
            roomSid: recordInfo.sid,
          });
          handleMsgFromChild(toChild.toJsonString(), child.pid);
        }
      }
    });
  };

  const handleMsgFromChild = async (m: string, pid: number) => {
    const msg = FromChildToParent.fromJsonString(m);
    let increment = true;

    const payload = new RecorderToPlugNmeet({
      from: 'recorder',
      status: msg.status,
      task: msg.task,
      msg: msg.msg,
      recordingId: msg.recordingId,
      roomSid: msg.roomSid,
      roomId: msg.roomId,
      recorderId: recorder.id, // this recorder ID
    });

    if (
      msg.task === RecordingTasks.END_RECORDING ||
      msg.task === RecordingTasks.END_RTMP
    ) {
      increment = false;
      let serviceType = RecorderServiceType.RECORDING;
      if (payload.task === RecordingTasks.END_RTMP) {
        serviceType = RecorderServiceType.RTMP;
      }
      // clean up
      childProcessesInfoMapByChildPid.delete(pid);
      childProcessesMapByRoomSid.delete(serviceType + ':' + payload.roomSid);
    }

    if (payload) {
      await notify(plugNmeetInfo, payload);
      await updateRecorderProgress(redis, recorder.id, increment);
    }
  };

  const startPing = () => {
    // send first ping
    sendPing(redis, recorder.id);
    // let's send ping in every 5 seconds
    // to make sure this node is online
    setInterval(() => {
      sendPing(redis, recorder.id);
    }, 5000);
  };
})();
