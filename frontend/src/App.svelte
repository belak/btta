<script lang="ts">
  import Mousetrap from 'mousetrap';
  import { scores, images, error, refreshScores, refreshImages } from './lib/stores';
  import { isMobile } from './lib/utils';
  import LeaderboardPage from './LeaderboardPage.svelte';
  import ImagePage from './ImagePage.svelte';

  type PageType = 'leaderboard' | 'images';

  let currentPage = $state<PageType>('leaderboard');
  let paused = $state(false);
  let imageCounter = $state(0);
  let windowWidth = $state(window.innerWidth);
  let navTrigger = $state(0);

  const onMobile = $derived(isMobile(windowWidth));

  function handleResize() {
    windowWidth = window.innerWidth;
  }

  function togglePaused() {
    paused = !paused;
  }

  function onNextPage() {
    if (currentPage === 'images') {
      currentPage = 'leaderboard';
      imageCounter += 1;
    }
  }

  function onFinished() {
    if (currentPage === 'leaderboard') {
      currentPage = 'images';
      if (!onMobile) refreshScores();
    } else if (currentPage === 'images') {
      currentPage = 'leaderboard';
      imageCounter = 0;
      if (!onMobile) refreshImages();
    }
  }

  // Startup race guard: avoid spinning between pages while data loads
  $effect(() => {
    const scoresLoaded = $scores.length;
    const imagesLoaded = $images.length;
    if (scoresLoaded === imagesLoaded) return;
    if (currentPage === 'leaderboard' && !scoresLoaded) {
      currentPage = 'images';
    } else if (currentPage === 'images' && !imagesLoaded) {
      currentPage = 'leaderboard';
    }
  });

  const showLeaderboard = $derived(!onMobile ? currentPage === 'leaderboard' : true);
  const showImages = $derived(!onMobile ? currentPage === 'images' : true);

  $effect(() => {
    function handleNav() { navTrigger++; }
    window.addEventListener('resize', handleResize);
    Mousetrap.bind('p', togglePaused);
    Mousetrap.bind('space', handleNav);
    Mousetrap.bind('enter', handleNav);
    Mousetrap.bind('right', handleNav);
    return () => {
      window.removeEventListener('resize', handleResize);
      Mousetrap.unbind('p');
      Mousetrap.unbind('space');
      Mousetrap.unbind('enter');
      Mousetrap.unbind('right');
    };
  });
</script>

<div class="App">
  <header>
    <img class="logo" src="logo.png" alt="Back to the Arcade" />
    <img class="leaderboard" src="leaderboard-text.png" alt="Leaderboard" />
  </header>

  {#if $error}
    <div>
      <h2>Error:</h2>
      {$error}
    </div>
  {/if}

  {#if $scores.length && showLeaderboard}
    <LeaderboardPage {onFinished} {onNextPage} {paused} {navTrigger} />
  {/if}

  {#if $images.length && showImages}
    <ImagePage {onFinished} {onNextPage} {paused} page={imageCounter} {navTrigger} />
  {/if}

  <footer>
    <img src="pacman-ghosts.jpg" alt="" />
  </footer>

  {#if paused}
    <span class="playPause">⏸</span>
  {:else}
    <span class="playPause fadeOut">▶</span>
  {/if}
</div>
