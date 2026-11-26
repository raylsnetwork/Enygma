"use strict";
// Tells the coordinator, during the participant's turn, that their browser
// is still contributing. The client's own worker cannot: its thread is busy
// with the contribution. A worker's timers keep running while the tab is in
// the background, where the page's own would be slowed. The page posts
// {url, token} to start and {} to stop. It posts to /v1/heartbeat, never
// /v1/turn: asking for a turn would queue a participant whose turn has
// passed on while their contribution is still under way.
let timer = null;

onmessage = (ev) => {
  clearInterval(timer);
  timer = null;
  const { url, token } = ev.data || {};
  if (!token) return;
  const beat = () =>
    fetch(url + "/v1/heartbeat", { method: "POST", headers: { Authorization: "Bearer " + token } }).catch(() => {});
  beat();
  timer = setInterval(beat, 10000);
};
