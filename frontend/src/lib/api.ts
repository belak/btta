import { differenceInSeconds, parseISO } from 'date-fns';
import { buildImageUrl } from './utils';

export type Score = {
  id: number;
  gameName: string;
  gameBanner: string;
  gameBannerThumbnail: string;
  playerName: string;
  playerScore: number;
  newScore: boolean;
};

export type Image = {
  name: string;
  image: string;
};

type ScoreAPIResponse = {
  id: number;
  game_name: string;
  game_banner: string;
  game_banner_thumbnail: string;
  player_name: string;
  player_score: number;
  created: string;
  modified: string;
};

const BASE_URL: string = import.meta.env.VITE_API_URL ?? '';

export async function fetchScores(): Promise<Score[]> {
  const resp = await fetch(`${BASE_URL}/api/scores/`);
  if (resp.status !== 200) {
    const text = await resp.text();
    throw new Error(`Failed to get scores: ${text}`);
  }
  const data = (await resp.json()) as ScoreAPIResponse[];
  const cur = new Date();
  return data.map((item) => {
    const modified = parseISO(item.modified);
    const created = parseISO(item.created);
    const newScore =
      modified > created &&
      differenceInSeconds(cur, modified) < 3600 * 24 * 30;
    return {
      id: item.id,
      gameName: item.game_name,
      gameBanner: item.game_banner,
      gameBannerThumbnail: item.game_banner_thumbnail,
      playerName: item.player_name,
      playerScore: item.player_score,
      newScore,
    };
  });
}

export async function fetchImages(): Promise<Image[]> {
  const resp = await fetch(`${BASE_URL}/api/images/`);
  if (resp.status !== 200) {
    const text = await resp.text();
    throw new Error(`Failed to get images: ${text}`);
  }
  return (await resp.json()) as Image[];
}
