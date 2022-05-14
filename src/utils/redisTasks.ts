import Redis, { RedisOptions } from 'ioredis';
import { RecorderRedisHashInfo } from './interfaces';
const recorderKey = 'pnm:recorders';

export const addRecorder = async (
  redisOptions: RedisOptions,
  recorder_id: string,
  max_limit: number,
) => {
  const redis = new Redis(redisOptions);
  const recorderInfo: any = {};
  recorderInfo[recorder_id] = JSON.stringify({
    maxLimit: max_limit,
    currentProgress: 0,
    last_ping: Date.now(),
    created: Date.now(),
  });
  await redis.hset(recorderKey, recorderInfo);
};

export const sendPing = async (
  redisOptions: RedisOptions,
  recorder_id: string,
) => {
  const redis = new Redis(redisOptions);
  const info = await redis.hget(recorderKey, recorder_id);

  if (!info) {
    return;
  }

  const currentInfo: RecorderRedisHashInfo = JSON.parse(info);
  currentInfo.last_ping = Date.now();

  // update again
  const recorderInfo: any = {};
  recorderInfo[recorder_id] = JSON.stringify(currentInfo);
  await redis.hset(recorderKey, recorderInfo);
};
