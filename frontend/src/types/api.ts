export interface Config {
  key: string;
  value: string;
  updated_at: string;
}

export interface AIRole {
  id: number;
  role_key: string;
  role_name: string;
  description: string;
  prompt: string;
  is_default: boolean;
  created_at: string;
  updated_at: string;
}

export interface AIModel {
  id: number;
  model_key: string;
  model_name: string;
  provider: string;
  description: string;
  input_price: number;
  output_price: number;
  is_default: boolean;
  created_at: string;
  updated_at: string;
}

export interface Bid {
  id: string;
  serial: string;
  title: string;
  buyer: string;
  area: string;
  city: string;
  district?: string;
  budget: number;
  industry: string;
  publish_time: number;
  bid_end_time: number;
  ai_analysis: string;
  ai_suitable: number;
  ai_score: number;
  ai_match_level: string;
  ai_priority: string;
  source: string;
  status: number;
  wechat_sent: number;
  wechat_sent_at: string;
  created_at: string;
  keywords: string;
  site: string;
  detail?: string;
  description?: string;
  pdf_url?: string;
  original_pdf_url?: string;
  original_href?: string;
  has_current_file?: boolean;
  has_original_file?: boolean;
  has_current_pdf?: boolean;
  has_original_pdf?: boolean;
  current_is_external?: boolean;
  current_external_url?: string;
  current_external_label?: string;
  original_is_external?: boolean;
  original_external_url?: string;
  original_external_label?: string;
  has_pdf_url?: boolean;
  has_original_pdf_url?: boolean;
  ai_model?: string;
  ai_thinking_process?: string;
  project_code?: string;
  project_name?: string;
  public_type?: string;
  top_type?: string;
  sub_type?: string;
  purchasing?: string;
  fetch_time?: string;
  html_url?: string;
  sourceOptions?: Bid[];
}

export interface ReanalyzeDebugResult {
  id: string;
  serial: string;
  analysis: Record<string, unknown>;
  ai_model: string;
  prompt: string;
  raw_response: string;
  thinking_process: string;
  analyzed_at: string;
}

export interface Statistics {
  total_bids: number;
  suitable_bids: number;
  avg_score: number;
  source_stats: Record<string, number>;
}

export interface WorkerStatistics {
  today: number;
  total: number;
  highPriority: number;
  suitable: number;
  notSuitable: number;
  trend: Array<{ date: string; count: number }>;
  regions: Array<{ area: string; count: number }>;
}

export interface AIScheduledTask {
  id: number;
  task_name: string;
  description: string;
  cron_expression: string;
  schedule_time: string;
  schedule_type: string;
  ai_role: string;
  ai_model: string;
  question: string;
  data_source: string;
  date_from: string;
  date_to: string;
  enable_wechat: boolean;
  wechat_room_id: string;
  is_active: boolean;
  last_run_at: string;
  last_run_status: string;
  last_run_result: string;
  next_run_at: string;
  total_runs: number;
  success_runs: number;
  failed_runs: number;
  created_at: string;
  updated_at: string;
}

export interface AITaskHistory {
  id: number;
  task_id: number;
  started_at: string;
  finished_at: string;
  status: string;
  ai_role: string;
  ai_model: string;
  question: string;
  data_count: number;
  ai_response: string;
  error_message: string;
  wechat_sent: boolean;
  wechat_result: string;
  created_at: string;
}

export interface WinnerInfo {
  Winner: string;
  WinnerAmount: number;
  TenderNoticeName: string;
  TenderNoticeURL: string;
  CandidateNoticeName: string;
  CandidateNoticeURL: string;
  CandidatePDFURL: string;
  CandidateNoticeTime: string;
  ResultNoticeName: string;
  ResultNoticeURL: string;
  ResultPDFURL: string;
  ResultNoticeTime: string;
  FullDetails: string;
  Candidates: string[];
  HasCandidate: boolean;
  HasResult: boolean;
}

export interface TrackedBid {
  id: string;
  title: string;
  serial?: string;
  buyer?: string;
  area?: string;
  city?: string;
  budget?: number;
  industry?: string;
  site?: string;
  source?: string;
  publish_time?: number;
  ai_score?: number;
  winner?: string;
  winner_amount?: number;
  winner_fetched?: boolean;
  winner_fetch_enabled?: boolean;
  winner_fetch_attempts?: number;
  winner_fetch_last_error?: string;
  winner_fetched_at?: string;
  last_check_time?: string;
  track_completed?: boolean;
  wechat_room_id?: string;
  winner_info?: Record<string, unknown>;
}

export interface CompanySummary {
  search_company: string;
  total_projects: number;
  last_notice_time: string;
  last_project_name?: string;
  last_win_bidder?: string;
  last_win_price?: string;
}

export interface CompanyAwardRecord {
  id: number;
  bulletin_id: string;
  project_name: string;
  win_bidder: string;
  win_price: string;
  notice_time: string;
  notice_url: string;
  details?: string;
  search_company?: string;
}

export interface SystemSettings {
  DB_HOST?: string;
  DB_PORT?: string;
  DB_NAME?: string;
  DB_USER?: string;
  DB_PASSWORD?: string;
  LISTEN_ADDR?: string;
  PORT?: string;
  RELAY_AI_BASE_URL: string;
  RELAY_AI_API_KEY: string;
  RELAY_AI_MODEL: string;
  RELAY_AI_FILE_MODEL: string;
  AI_PROVIDER: string;
  CTYUN_MODEL: string;
  ai_prompt: string;
  access_password: string;
  company_auto_fetch_time: string;
  auto_exclude_days: string;
  auto_delete_days: string;
  AUTO_AI_ANALYSIS_ENABLED: string | boolean;
  WECHAT_HIGH_SCORE_THRESHOLD: string;
  WECHAT_HOOK_URL: string;
  WECHAT_NOTICE_BASE_URL: string;
  WECHAT_HIGH_SCORE_ROOM: string;
  WECHAT_DEFAULT_ROOM: string;
  ai_logic: string;
}
