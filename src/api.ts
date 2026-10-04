import type { ConfigResponse, RecommendResponse, SavedMovie, SearchResponse } from './types';

async function api<T>(path: string, options?: RequestInit): Promise<T> {
    const response = await fetch(path, options);
    const data = await response.json();
    if (!response.ok) throw new Error(data.error || '通信に失敗しました。');
    return data as T;
}

export const fetchConfig = () => api<ConfigResponse>('/api/config');

export const searchMovies = (query: string) => api<SearchResponse>(`/api/search?q=${encodeURIComponent(query)}`);

export const recommendMovie = (mood: string, library: SavedMovie[]) => api<RecommendResponse>('/api/recommend', {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ mood, library })
});
