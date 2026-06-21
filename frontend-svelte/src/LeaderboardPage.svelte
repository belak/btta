<script lang="ts">
  import { scores } from './lib/stores';
  import { isMobile } from './lib/utils';

  interface Props {
    onFinished: () => void;
    onNextPage: () => void;
    paused: boolean;
    navTrigger: number;
  }
  const { onFinished, onNextPage, paused, navTrigger }: Props = $props();
  let prevNavTrigger = navTrigger;

  let offset = $state(0);
  let count = $state(1);
  let windowWidth = $state(window.innerWidth);
  let resetKey = $state(0);
  let scoresRef: HTMLDivElement | undefined = $state();

  function handleResize() {
    windowWidth = window.innerWidth;
    if (!isMobile(windowWidth)) {
      count = 1;
    }
  }

  // Preload thumbnails on desktop when scores change
  $effect(() => {
    if (!isMobile(windowWidth) && $scores.length > 0) {
      $scores.forEach(({ gameBannerThumbnail }) => {
        const img = new Image();
        img.src = gameBannerThumbnail;
      });
    }
  });

  // If scores empty, call onFinished
  $effect(() => {
    if ($scores.length === 0) {
      onFinished();
    }
  });

  // Measure how many rows fit in the container
  function measureCount() {
    if (!scoresRef) return;
    if (isMobile(windowWidth)) {
      count = $scores.length;
      return;
    }
    const containerHeight = scoresRef.clientHeight;
    const firstScore = scoresRef.firstElementChild?.firstElementChild as HTMLElement | null;
    if (!firstScore) return;
    const newCount = Math.floor(containerHeight / (firstScore.clientHeight + 1));
    if (newCount > 0 && count !== newCount) {
      count = newCount;
    }
  }

  // Re-measure when window size or scores change
  $effect(() => {
    windowWidth;
    $scores.length;
    // Use microtask to let DOM update first
    queueMicrotask(measureCount);
  });

  const visibleScores = $derived($scores.slice(offset, offset + count));

  function nextPage() {
    const newOffset = offset + count;
    const finalOffset = newOffset >= $scores.length ? 0 : newOffset;
    if (finalOffset !== offset) {
      offset = finalOffset;
    }
    if (finalOffset === 0) {
      onFinished();
    } else {
      onNextPage();
    }
  }

  // Auto-advance timer; resets when resetKey increments or paused changes
  $effect(() => {
    resetKey; // declare dependency
    if (paused) return;
    const interval = setInterval(nextPage, 9000);
    return () => clearInterval(interval);
  });

  // Nav key pressed in App — advance and reset timer
  $effect(() => {
    if (navTrigger !== prevNavTrigger) {
      prevNavTrigger = navTrigger;
      nextPage();
      resetKey++;
    }
  });

  $effect(() => {
    window.addEventListener('resize', handleResize);
    return () => window.removeEventListener('resize', handleResize);
  });
</script>

<div class="scoresContainer" bind:this={scoresRef}>
  <div class="scores">
    {#each visibleScores as item, idx}
      {#if idx !== 0}
        <span class="line"></span>
      {/if}
      <span class="gameName" class:newScore={item.newScore}>
        <img src={item.gameBannerThumbnail} alt={item.gameName} />
      </span>
      <span class="playerName" class:newScore={item.newScore}>
        {item.playerName}
      </span>
      <span class="score" class:newScore={item.newScore}>
        {Number(item.playerScore).toLocaleString()}
      </span>
    {/each}
  </div>
</div>
