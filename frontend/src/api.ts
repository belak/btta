import { differenceInSeconds, parseISO } from "date-fns";

type Score = {
  id: number;
  gameName: string;
  gameBanner: string;
  gameBannerThumbnail: string;
  playerName: string;
  playerScore: number;
  newScore: boolean;
};

const fetchScores = async (baseURL: string): Promise<Score[]> => {
  try {
    const resp = await fetch(`${baseURL}/api/scores/`);
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
          gameBanner: item.game_banner,
          gameBannerThumbnail: item.game_banner_thumbnail,
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

const fetchImages = async (baseURL: string): Promise<Image[]> => {
  try {
    const resp = await fetch(`${baseURL}/api/images/`);
    if (resp.status === 200) {
      return await resp.json();
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
