import { Kvm } from '@nats-io/kv';
import { logger } from './helper';

const prefix = 'pnm-';
const RecordersKvKey = prefix + 'recorders';
const RecordersInfoKvkey = prefix + 'recorderInfo-';
const RecorderMaxLimitKey = 'max_limit';
const RecorderCurrenProgressKey = 'current_progress';

export const sendPing = async (kvm: Kvm, recorderId: string) => {
  try {
    const kv = await kvm.create(RecordersKvKey);
    const now = new Date().getUTCMilliseconds();
    await kv.put(recorderId, `${now}`);
  } catch (error) {
    logger.error('Error: ', error);
  }
};

export const addRecorder = async (
  kvm: Kvm,
  recorderId: string,
  max_limit: number,
) => {
  try {
    const kv = await kvm.create(RecordersInfoKvkey + recorderId);
    await kv.put(RecorderMaxLimitKey, `${max_limit}`);
    await kv.put(RecorderCurrenProgressKey, `0`);
  } catch (error) {
    logger.error('Error: ', error);
  }
};

export const updateRecorderProgress = async (
  kvm: Kvm,
  recorderId: string,
  increment: boolean,
) => {
  try {
    const kv = await kvm.open(RecordersInfoKvkey + recorderId);
    const entry = await kv.get(RecorderCurrenProgressKey);
    if (entry) {
      let current = Number(entry.string());
      if (increment) {
        current++;
      } else {
        current--;
      }
      await kv.put(RecorderCurrenProgressKey, `${current}`);
    }
  } catch (error) {
    logger.error('Error: ', error);
  }
};
