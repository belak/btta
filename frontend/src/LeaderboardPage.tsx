import React, {
  useEffect,
  useLayoutEffect,
  useState,
  useRef,
  useCallback,
} from "react";

import { useWindowSize, isMobile, useNextPage } from "./utils";
import useAPIState from "./useAPIState";

const Leaderboard = ({
  onFinished,
  onNextPage,
  paused,
}: {
  onFinished: () => void;
  onNextPage: () => void;
  paused: boolean;
}) => {
  const [offset, setOffset] = useState(0);
  const [count, setCount] = useState(1);

  const { scores } = useAPIState();

  useEffect(() => {
    if (scores.length === 0) {
      onFinished();
    }

    scores.map(({ gameBannerThumbnail: src }) => {
      let image = new Image();
      image.src = src;
      return image;
    });
  }, [scores, onFinished]);

  const scoresRef = useRef<HTMLDivElement>(null);

  const nextPage = useCallback(() => {
    const newOffset = offset + count;
    const finalOffset = newOffset >= scores.length ? 0 : newOffset;
    if (finalOffset !== offset) {
      setOffset(finalOffset);
    }

    if (finalOffset === 0) {
      onFinished();
    } else {
      onNextPage();
    }
  }, [offset, count, scores.length, setOffset, onFinished, onNextPage]);

  useNextPage(nextPage, paused);

  const windowSize = useWindowSize();
  useLayoutEffect(() => {
    if (!isMobile(windowSize)) {
      setCount(1);
    }
  }, [windowSize, setCount]);

  useLayoutEffect(() => {
    if (!scoresRef.current) {
      return;
    }

    const width = windowSize.width || 0;

    if (width < 1000) {
      setCount(scores.length);
      return;
    }

    const containerHeight = scoresRef.current.clientHeight;

    const firstScore = scoresRef.current.firstChild
      ?.firstChild as HTMLSpanElement | null;
    if (!firstScore) {
      return;
    }

    const newCount = Math.floor(
      containerHeight / (firstScore.clientHeight + 1)
    );
    if (count !== newCount) {
      setCount(newCount);
    }
  }, [count, setCount, scoresRef, scores.length, windowSize]);

  return (
    <div className="scoresContainer" ref={scoresRef}>
      <div className="scores">
        {scores.slice(offset, offset + count).map((item, idx) => {
          const ns = item.newScore ? " newScore" : "";
          return (
            <React.Fragment key={item.id}>
              {idx !== 0 && <span className="line" />}
              <span className={`gameName${ns}`}>
                <img src={item.gameBannerThumbnail} alt={item.gameName} />
              </span>
              <span className={`playerName${ns}`}>
                {item.playerName}
              </span>
              <span className={`score${ns}`}>
                {Number(item.playerScore).toLocaleString()}
              </span>
            </React.Fragment>
          );
        })}
      </div>
    </div>
  );
};

export default Leaderboard;
