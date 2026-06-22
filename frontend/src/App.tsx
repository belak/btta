import { useState, useCallback, useEffect } from "react";

import Mousetrap from "mousetrap";

import LeaderboardPage from "./LeaderboardPage";
import useAPIState from "./useAPIState";
import ImagePage from "./ImagePage";
import { useWindowSize, isMobile } from "./utils";

type PageType = "leaderboard" | "images";

function App() {
  const { images, scores, error, refreshImages, refreshScores } = useAPIState();

  const [paused, setPaused] = useState(false);
  const [currentPage, setCurrentPage] = useState<PageType>("images");
  const [imageCounter, setImageCounter] = useState(0);

  const windowSize = useWindowSize();
  const onMobile = isMobile(windowSize);

  const togglePaused = useCallback(() => {
    setPaused((p) => !p);
  }, []);

  useEffect(() => {
    Mousetrap.bind("p", togglePaused);
    return () => { Mousetrap.unbind("p"); };
  }, [togglePaused]);

  const onNextPage = useCallback(() => {
    if (currentPage === "images") {
      setCurrentPage("leaderboard");
      setImageCounter((c) => c + 1);
    }
  }, [currentPage]);

  const onFinished = useCallback(() => {
    switch (currentPage) {
      case "leaderboard":
        setCurrentPage("images");
        if (!onMobile) refreshScores();
        break;
      case "images":
        setCurrentPage("leaderboard");
        setImageCounter(0);
        if (!onMobile) refreshImages();
        break;
      default:
        setCurrentPage("leaderboard");
        break;
    }
  }, [currentPage, refreshImages, refreshScores, onMobile]);

  const onLeaderboard = !onMobile ? currentPage === "leaderboard" : true;
  const onImages = !onMobile ? currentPage === "images" : true;

  const scoresLoaded = scores.length;
  const imagesLoaded = images.length;

  // Avoid spinning between pages while loading if only one is ready.
  useEffect(() => {
    if (scoresLoaded === imagesLoaded) return;
    if (currentPage === "leaderboard" && !scoresLoaded) {
      setCurrentPage("images");
    } else if (currentPage === "images" && !imagesLoaded) {
      setCurrentPage("leaderboard");
    }
  }, [scoresLoaded, imagesLoaded, currentPage]);

  return (
    <div className="App">
      <header>
        <img className="logo" src="logo.png" alt="Back to the Arcade" />
        <img
          className="leaderboard"
          src="leaderboard-text.png"
          alt="Leaderboard"
        />
      </header>

      {error && (
        <div>
          <h2>Error:</h2>
          {error}
        </div>
      )}

      {scoresLoaded && onLeaderboard && (
        <LeaderboardPage
          onFinished={onFinished}
          onNextPage={onNextPage}
          paused={paused}
        />
      )}
      {imagesLoaded && onImages && (
        <ImagePage
          onFinished={onFinished}
          onNextPage={onNextPage}
          paused={paused}
          page={imageCounter}
        />
      )}

      <footer>
        <img src="pacman-ghosts.jpg" alt="" />
      </footer>

      <span className={`playPause${paused ? "" : " fadeOut"}`}>
        {paused ? "⏸" : "▶"}
      </span>
    </div>
  );
}

export default App;
