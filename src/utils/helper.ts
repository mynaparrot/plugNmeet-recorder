import { createLogger, transports, format } from 'winston';
import { createHmac } from 'crypto';
import axios from 'axios';
import axiosRetry from 'axios-retry';

import { PlugNmeetInfo } from './interfaces';
import { RecorderToPlugNmeet } from '../proto/plugnmeet_recorder_pb';
const { combine, timestamp, printf } = format;

axiosRetry(axios, { retryDelay: axiosRetry.exponentialDelay, retries: 4 });

const logFormat = printf(({ level, message, timestamp }) => {
  return `${timestamp} ${level}: ${message}`;
});

export const logger = createLogger({
  format: combine(timestamp(), logFormat),
  transports: [new transports.Console()],
});

export const sleep = (ms: number) => {
  return new Promise((resolve) => setTimeout(resolve, ms));
};

export const notify = async (
  plugNmeetInfo: PlugNmeetInfo,
  body: RecorderToPlugNmeet,
) => {
  try {
    const b = body.toBinary();
    const signature = createHmac('sha256', plugNmeetInfo.api_secret)
      .update(b)
      .digest('hex');

    const url = plugNmeetInfo.host + '/auth/recorder/notify';
    const res = await axios.post(url, b, {
      headers: {
        'API-KEY': plugNmeetInfo.api_key,
        'HASH-SIGNATURE': signature,
        'Content-Type': 'application/protobuf',
      },
    });
    return res.data;
  } catch (e: any) {
    logger.error(e);
  }
};
