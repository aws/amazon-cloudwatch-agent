import React from "react";
import ReactDOM from "react-dom/client";
import { App } from "./common/App";
import { BrowserRouter } from "react-router-dom";
import { RecoilRoot } from "recoil";

const container = document.getElementById("root") as HTMLElement;

ReactDOM.createRoot(container).render(
  <React.StrictMode>
    <RecoilRoot>
      <BrowserRouter>
        <App />
      </BrowserRouter>
    </RecoilRoot>
  </React.StrictMode>
);
