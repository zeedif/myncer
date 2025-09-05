const config = {
  spotifyClientId: window.MYNCER_CONFIG?.VITE_SPOTIFY_CLIENT_ID ?? import.meta.env.VITE_SPOTIFY_CLIENT_ID,
  spotifyRedirectUri: window.MYNCER_CONFIG?.VITE_SPOTIFY_REDIRECT_URI ?? import.meta.env.VITE_SPOTIFY_REDIRECT_URI,
  youtubeClientId: window.MYNCER_CONFIG?.VITE_YOUTUBE_CLIENT_ID ?? import.meta.env.VITE_YOUTUBE_CLIENT_ID,
  youtubeRedirectUri: window.MYNCER_CONFIG?.VITE_YOUTUBE_REDIRECT_URI ?? import.meta.env.VITE_YOUTUBE_REDIRECT_URI,
  tidalClientId: window.MYNCER_CONFIG?.VITE_TIDAL_CLIENT_ID ?? import.meta.env.VITE_TIDAL_CLIENT_ID,
  tidalRedirectUri: window.MYNCER_CONFIG?.VITE_TIDAL_REDIRECT_URI ?? import.meta.env.VITE_TIDAL_REDIRECT_URI,
};

export default config;
