import { Kvm } from '@nats-io/kv';
import { RecorderInfoKeys } from 'plugnmeet-protocol-js';
import { logger } from './helper';

export const sendPing = async (kvm: Kvm, keyName: string) => {
  //recorder.recorder_info_kv + '-' + recorderId
  try {
    const kv = await kvm.create(keyName);
    const now = Date.now();
    await kv.put(RecorderInfoKeys.RECORDER_INFO_LAST_PING.toString(), `${now}`);
  } catch (error) {
    logger.error('Error: ', error);
  }
};

export const addRecorder = async (
  kvm: Kvm,
  keyName: string,
  max_limit: number,
) => {
  try {
    const kv = await kvm.create(keyName);
    const now = Date.now();
    await kv.put(
      RecorderInfoKeys.RECORDER_INFO_MAX_LIMIT.toString(),
      `${max_limit}`,
    );
    await kv.put(
      RecorderInfoKeys.RECORDER_INFO_CURRENT_PROGRESS.toString(),
      `0`,
    );
    await kv.put(
      RecorderInfoKeys.RECORDER_INFO_LAST_PING.toString(),
      now.toString(),
    );
  } catch (error) {
    logger.error('Error: ', error);
  }
};

export const updateRecorderProgress = async (
  kvm: Kvm,
  keyName: string,
  increment: boolean,
) => {
  try {
    const kv = await kvm.open(keyName);
    const entry = await kv.get(
      RecorderInfoKeys.RECORDER_INFO_CURRENT_PROGRESS.toString(),
    );
    if (entry) {
      let current = Number(entry.string());
      if (increment) {
        current++;
      } else {
        current--;
      }
      await kv.put(
        RecorderInfoKeys.RECORDER_INFO_CURRENT_PROGRESS.toString(),
        `${current}`,
      );
    }
  } catch (error) {
    logger.error('Error: ', error);
  }
};
