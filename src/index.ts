import Redis, { RedisOptions } from 'ioredis';
import yaml from 'js-yaml';
import * as fs from 'fs';
import { fork } from 'child_process';

import {
  WebsocketServerInfo,
  PlugNmeetInfo,
  Recorder,
  RecorderArgs,
  RecorderReq,
  RedisInfo,
} from './utils/interfaces';
import { logger } from './utils/helper';
import { addRecorder, sendPing } from './utils/redisTasks';

let redisInfo: RedisInfo;
let recorder: Recorder;
let plugNmeetInfo: PlugNmeetInfo;
let websocketServerInfo: WebsocketServerInfo;

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

(async () => {
  const redisOptions: RedisOptions = {
    host: redisInfo.host,
    port: redisInfo.port,
    username: redisInfo.username,
    password: redisInfo.password,
    db: redisInfo.db,
    name: recorder.id,
  };
  const subNode = new Redis(redisOptions);

  subNode.subscribe('plug-n-meet-recorder', async (err) => {
    if (err) {
      logger.error('Failed to subscribe: %s', err.message);
    } else {
      logger.info('Subscribed successfully! Waiting for message');
      await addRecorder(redisOptions, recorder.id, recorder.max_limit);
      startPing();
    }
  });

  subNode.on('message', (channel, message) => {
    const payload: RecorderReq = JSON.parse(message);

    if (
      payload.from === 'plugnmeet' &&
      (payload.task === 'start-recording' || payload.task === 'start-rtmp') &&
      payload.recorder_id === recorder.id
    ) {
      logger.info('Main: ' + payload.task);
      const websocket_url = `${websocketServerInfo.host}:${websocketServerInfo.port}?auth_token=${websocketServerInfo.auth_token}&room_id=${payload.room_id}&room_sid=${payload.sid}&record_id=${payload.record_id}`;

      const toSend: RecorderArgs = {
        room_id: payload.room_id,
        record_id: payload.record_id,
        sid: payload.sid,
        access_token: payload.access_token,
        redisInfo: redisInfo,
        plugNmeetInfo: plugNmeetInfo,
        post_mp4_convert: recorder.post_mp4_convert,
        copy_to_path: recorder.copy_to_path,
        recorder_id: recorder.id,
        serviceType: 'recording',
        websocket_url,
      };

      if (recorder.custom_chrome_path) {
        toSend.custom_chrome_path = recorder.custom_chrome_path;
      }

      if (payload.task === 'start-recording') {
        toSend.websocket_url = toSend.websocket_url + '&service=recording';
      } else if (payload.task === 'start-rtmp') {
        toSend.websocket_url =
          toSend.websocket_url + '&service=rtmp&rtmp_url=' + payload.rtmp_url;
        toSend.serviceType = 'rtmp';
      }

      if (typeof process.env.TS_NODE_DEV !== 'undefined') {
        fork('src/recorder', [JSON.stringify(toSend)], {
          execArgv: ['-r', 'ts-node/register'],
        });
      } else {
        fork('dist/recorder', [JSON.stringify(toSend)]);
      }
    }
  });

  const startPing = () => {
    // send first ping
    sendPing(redisOptions, recorder.id);
    // let's send ping in every 5 seconds
    // to make sure this node is online
    setInterval(() => {
      sendPing(redisOptions, recorder.id);
    }, 5000);
  };
})();
