import Redis from 'ioredis';
import { RecorderRedisHashInfo } from './interfaces';
import { logger } from './helper';

const recorderKey = 'pnm:recorders';

export const addRecorder = async (
  redis: Redis,
  recorder_id: string,
  max_limit: number,
) => {
  try {
    const now = Math.floor(new Date().getTime() / 1000);
    const recorderInfo: any = {};
    recorderInfo[recorder_id] = JSON.stringify({
      maxLimit: max_limit,
      currentProgress: 0,
      lastPing: now,
      created: now,
    });

    await redis.hset(recorderKey, recorderInfo);
  } catch (e) {
    logger.error(e);
  }
};

export const sendPing = async (redis: Redis, recorder_id: string) => {
  let watch = '';
  try {
    watch = await redis.watch(recorderKey);
    if (watch !== 'OK') {
      return;
    }
    const info = await redis.hget(recorderKey, recorder_id);
    if (!info) {
      return;
    }

    const currentInfo: RecorderRedisHashInfo = JSON.parse(info);
    currentInfo.lastPing = Math.floor(new Date().getTime() / 1000);

    // update again
    const r = redis.multi({ pipeline: true });
    const recorderInfo: any = {};
    recorderInfo[recorder_id] = JSON.stringify(currentInfo);
    await r.hset(recorderKey, recorderInfo);
    await r.exec();

    await redis.unwatch();
  } catch (e) {
    logger.error(e);
  } finally {
    if (watch === 'OK') {
      await redis.unwatch();
    }
  }
};

export const updateRecorderProgress = async (
  redis: Redis,
  recorder_id: any,
  increment: boolean,
) => {
  let watch = '';
  try {
    watch = await redis.watch(recorderKey);
    if (watch !== 'OK') {
      return;
    }
    const info = await redis.hget(recorderKey, recorder_id);
    if (!info) {
      return;
    }

    const currentInfo: RecorderRedisHashInfo = JSON.parse(info);
    if (increment) {
      currentInfo.currentProgress += 1;
    } else if (currentInfo.currentProgress > 0) {
      currentInfo.currentProgress -= 1;
    }

    const r = redis.multi({ pipeline: true });
    const recorderInfo: any = {};
    recorderInfo[recorder_id] = JSON.stringify(currentInfo);
    await r.hset(recorderKey, recorderInfo);
    await r.exec();
  } catch (e) {
    logger.error(e);
  } finally {
    if (watch === 'OK') {
      await redis.unwatch();
    }
  }
};
