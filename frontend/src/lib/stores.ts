import { writable } from 'svelte/store';
import { fetchScores, fetchImages } from './api';
import type { Score, Image } from './api';

export const scores = writable<Score[]>([]);
export const images = writable<Image[]>([]);
export const error = writable<string | undefined>(undefined);

export async function refreshScores(): Promise<void> {
  try {
    const data = await fetchScores();
    scores.set(data);
    error.set(undefined);
  } catch (e) {
    error.set(String(e));
  }
}

export async function refreshImages(): Promise<void> {
  try {
    const data = await fetchImages();
    images.set(data);
    error.set(undefined);
  } catch (e) {
    error.set(String(e));
  }
}

refreshScores();
refreshImages();
