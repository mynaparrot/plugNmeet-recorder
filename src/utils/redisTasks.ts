import Redis, { RedisOptions } from 'ioredis';
import { RecorderRedisHashInfo, RedisInfo } from './interfaces';
import { logger } from './helper';
import { SentinelAddress } from 'ioredis/built/connectors/SentinelConnector/types';

const recorderKey = 'pnm:recorders';

export const openRedisConnection = async (redisInfo: RedisInfo) => {
  let redisOptions: RedisOptions = {
    username: redisInfo.username ?? '',
    password: redisInfo.password ?? '',
    db: redisInfo.db ?? 0,
  };
  if (!redisInfo.use_tls) {
    redisOptions.host = redisInfo.host;
    redisOptions.port = redisInfo.port;
  } else {
    redisOptions.tls = {
      host: redisInfo.host,
      port: redisInfo.port,
    };
  }

  if (redisInfo.sentinel_addresses && redisInfo.sentinel_addresses.length > 0) {
    const sentinel_addresses: Array<SentinelAddress> = [];
    redisInfo.sentinel_addresses.forEach((a) => {
      const parts = a.split(':');
      const address: SentinelAddress = {
        host: parts[0],
        port: Number(parts[1]),
      };
      sentinel_addresses.push(address);
    });

    redisOptions = {
      username: redisInfo.username ?? '',
      password: redisInfo.password ?? '',
      db: redisInfo.db ?? 0,
      name: redisInfo.sentinel_master_name,
      sentinels: sentinel_addresses,
      sentinelUsername: redisInfo.sentinel_username ?? '',
      sentinelPassword: redisInfo.sentinel_password ?? '',
    };

    if (redisInfo.use_tls) {
      redisOptions.enableTLSForSentinelMode = true;
    }
  }

  let redis: Redis | null = null;

  try {
    redis = new Redis(redisOptions);
    const ping = await redis.ping();
    if (ping !== 'PONG') {
      return null;
    }
  } catch (e) {
    logger.error(e);
  }

  return redis;
};

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
