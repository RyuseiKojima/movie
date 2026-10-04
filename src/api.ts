import type { ConfigResponse, Movie, RecommendResponse, SavedMovie, SearchResponse, YearRange } from './types';

async function api<T>(path: string, options?: RequestInit): Promise<T> {
    const response = await fetch(path, options);
    const data = await response.json();
    if (!response.ok) throw new Error(data.error || '通信に失敗しました。');
    return data as T;
}

export const fetchConfig = () => api<ConfigResponse>('/api/config');

export function searchMovies(query: string, { yearFrom, yearTo }: YearRange) {
    const params = new URLSearchParams({ q: query });
    if (yearFrom !== null) params.set('yearFrom', String(yearFrom));
    if (yearTo !== null) params.set('yearTo', String(yearTo));
    return api<SearchResponse>(`/api/search?${params}`);
}

export const recommendMovie = (mood: string, library: SavedMovie[], candidates: Movie[]) => api<RecommendResponse>('/api/recommend', {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ mood, library, candidates })
});
