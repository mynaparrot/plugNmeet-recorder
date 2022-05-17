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
  redis.watch(recorderKey, async () => {
    const info = await redis.hget(recorderKey, recorder_id);
    //console.log(info);
    if (!info) {
      return;
    }

    const currentInfo: RecorderRedisHashInfo = JSON.parse(info);
    currentInfo.last_ping = Date.now();

    // update again
    const r = redis.multi({ pipeline: true });
    const recorderInfo: any = {};
    recorderInfo[recorder_id] = JSON.stringify(currentInfo);
    await r.hset(recorderKey, recorderInfo);
    await r.exec();

    await redis.unwatch();
    await redis.quit();
  });
};

export const updateRecorderProgress = async (
  redisOptions: RedisOptions,
  recorder_id: any,
  increment: boolean,
) => {
  const redis = new Redis(redisOptions);
  redis.watch(recorderKey, async () => {
    const info = await redis.hget(recorderKey, recorder_id);
    if (!info) {
      return;
    }

    const currentInfo: RecorderRedisHashInfo = JSON.parse(info);
    if (increment) {
      currentInfo.currentProgress += 1;
    } else {
      currentInfo.currentProgress -= 1;
    }

    const r = redis.multi({ pipeline: true });
    const recorderInfo: any = {};
    recorderInfo[recorder_id] = JSON.stringify(currentInfo);
    await r.hset(recorderKey, recorderInfo);
    await r.exec();

    await redis.unwatch();
    await redis.quit();
  });
};
