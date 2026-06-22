import { differenceInSeconds, parseISO } from "date-fns";

const BASE_URL = import.meta.env.VITE_API_BASE_URL ?? "";

type Score = {
  id: number;
  gameName: string;
  gameBanner: string;
  gameBannerThumbnail: string;
  playerName: string;
  playerScore: number;
  newScore: boolean;
};

const fetchScores = async (): Promise<Score[]> => {
  try {
    const resp = await fetch(`${BASE_URL}/api/scores/`);
    if (resp.status === 200) {
      const data = await resp.json();
      const cur = new Date();

      return data.map((item: any) => {
        const modified = parseISO(item.modified);
        const created = parseISO(item.created);
        const newScore =
          modified > created &&
          differenceInSeconds(cur, modified) < 3600 * 24 * 30;
        return {
          id: item.id,
          gameName: item.game_name,
          gameBanner: BASE_URL + item.game_banner,
          gameBannerThumbnail: BASE_URL + item.game_banner_thumbnail,
          playerName: item.player_name,
          playerScore: item.player_score,
          newScore,
        };
      });
    } else {
      const text = await resp.text();
      throw "Failed to get scores: " + text;
    }
  } catch (e) {
    throw "Failed to get scores: " + e;
  }
};

type Image = {
  name: string;
  image: string;
};

const fetchImages = async (): Promise<Image[]> => {
  try {
    const resp = await fetch(`${BASE_URL}/api/images/`);
    if (resp.status === 200) {
      const data: Image[] = await resp.json();
      return data.map((item) => ({ ...item, image: BASE_URL + item.image }));
    } else {
      const text = await resp.text();
      throw "Failed to get images: " + text;
    }
  } catch (e) {
    throw "Failed to get images: " + e;
  }
};

export type { Score, Image };

export { fetchImages, fetchScores };
