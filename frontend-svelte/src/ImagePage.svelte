<script lang="ts">
  import { images } from './lib/stores';
  import { isMobile } from './lib/utils';

  interface Props {
    onFinished: () => void;
    onNextPage: () => void;
    paused: boolean;
    page: number;
    navTrigger: number;
  }
  const { onFinished, onNextPage, paused, page, navTrigger }: Props = $props();
  let prevNavTrigger = navTrigger;

  let windowWidth = $state(window.innerWidth);
  let resetKey = $state(0);

  function handleResize() {
    windowWidth = window.innerWidth;
  }

  // Preload all images on desktop when images store changes
  $effect(() => {
    if (!isMobile(windowWidth) && $images.length > 0) {
      $images.forEach(({ image: src }) => {
        const img = new Image();
        img.src = src;
      });
    }
  });

  const offset = $derived($images.length ? page % $images.length : 0);
  const currentImage = $derived($images[offset]);

  // If offset is out of range, call onFinished
  $effect(() => {
    if ($images.length > 0 && $images.length <= offset) {
      onFinished();
    }
  });

  // Auto-advance timer; resets when resetKey increments or paused changes
  $effect(() => {
    resetKey; // declare dependency so effect re-runs on reset
    if (paused) return;
    const interval = setInterval(() => onNextPage(), 9000);
    return () => clearInterval(interval);
  });

  // Nav key pressed in App — advance and reset timer
  $effect(() => {
    if (navTrigger !== prevNavTrigger) {
      prevNavTrigger = navTrigger;
      onNextPage();
      resetKey++;
    }
  });

  $effect(() => {
    window.addEventListener('resize', handleResize);
    return () => window.removeEventListener('resize', handleResize);
  });
</script>

<div class="imageContainer">
  {#if currentImage}
    <img src={currentImage.image} alt={currentImage.name} class="fullscreenImage" />
  {/if}
</div>
