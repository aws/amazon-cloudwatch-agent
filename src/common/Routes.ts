import * as React from "react";

export const HomePage = React.lazy(
  () => import("../containers/Homepage/index")
);
export const PerformanceReport = React.lazy(
  () => import("../containers/PerformanceReport/index")
);
export const PerformanceTrend = React.lazy(
  () => import("../containers/PerformanceTrend/index")
);
export const Wikipedia = React.lazy(
  () => import("../containers/Wikipedia/index")
);