import { createLogger, transports, format } from 'winston';
import { createHmac } from 'crypto';
import axios from 'axios';
import axiosRetry from 'axios-retry';
import DailyRotateFile from 'winston-daily-rotate-file';
import { toBinary } from '@bufbuild/protobuf';
import {
  type RecorderToPlugNmeet,
  RecorderToPlugNmeetSchema,
} from 'plugnmeet-protocol-js';

import { FFMPEGOptions, PlugNmeetInfo } from './interfaces';

const { combine, timestamp, printf } = format;

axiosRetry(axios, { retryDelay: axiosRetry.exponentialDelay, retries: 4 });

const logFormat = printf(({ level, message, timestamp }) => {
  return `${timestamp} ${level}: ${message}`;
});

const transportFile: DailyRotateFile = new DailyRotateFile({
  filename: './logs/recorder-%DATE%.log',
  datePattern: 'YYYY-MM-DD',
  zippedArchive: false,
  maxSize: '10m',
  maxFiles: '7d',
});

export const logger = createLogger({
  format: combine(timestamp(), logFormat),
  transports: [new transports.Console(), transportFile],
});

export const sleep = (ms: number) => {
  return new Promise((resolve) => setTimeout(resolve, ms));
};

export const notify = async (
  plugNmeetInfo: PlugNmeetInfo,
  body: RecorderToPlugNmeet,
) => {
  try {
    const b = toBinary(RecorderToPlugNmeetSchema, body);
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

export const getDefaultFFMPEGOptions = (): FFMPEGOptions => {
  return {
    recording: {
      pre_input: '',
      post_input: '-movflags faststart -c:v copy -preset veryfast', // we can copy as Chrome will record in h264 codec
    },
    rtmp: {
      pre_input: '',
      post_input:
        '-c:v libx264 -x264-params keyint=120:scenecut=0 -b:v 2500k -video_size 1280x720 -c:a aac -b:a 128k -ar 44100 -af highpass=f=200,lowpass=f=2000,afftdn -preset ultrafast -crf 5 -vf format=yuv420p -tune zerolatency',
    },
  };
};
