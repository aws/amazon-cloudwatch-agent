import React from 'react';
import ReactDOM from 'react-dom/client';
import { App } from './common/App';
import { BrowserRouter } from "react-router-dom";
import { RecoilRoot } from "recoil";
import reportWebVitals from './reportWebVitals';

const container = document.getElementById('root') as HTMLElement

ReactDOM.createRoot(container).render(
  <React.StrictMode>
    <RecoilRoot>
      <BrowserRouter>
        <App />
      </BrowserRouter>
    </RecoilRoot>
  </React.StrictMode>
);

// If you want to start measuring performance in your app, pass a function
// to log results (for example: reportWebVitals(console.log))
// or send to an analytics endpoint. Learn more: https://bit.ly/CRA-vitals
reportWebVitals();
