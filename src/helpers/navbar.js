import { Link } from "react-router-dom";
import React from "react";
import "./nav.css";
import { FaHome, FaTable, FaChartLine, FaBookOpen } from "react-icons/fa";
import logo from "../icons/inverselogo.png";

// This component handles navigation between page components.
export default function Navbar(props) {
  return (
    <div class="navbar">
      <ul>
        <li>
          <img src={logo} class="logo" alt="CloudWatch logo" />
        </li>
        <li>
          <Link to="/">
            <FaHome />
            Home
          </Link>
        </li>
        <li>
          <Link to="/table">
            <FaTable />
            Table
          </Link>
        </li>
        <li>
          <Link to="/graphics">
            <FaChartLine />
            Graph
          </Link>
        </li>
        <li>
          <Link to="/wiki">
            <FaBookOpen />
            Wiki&nbsp;&nbsp;&nbsp;&nbsp;
          </Link>
        </li>
        <li className="right">
          <div className="sync_info">
            <label
              style={{ backgroundColor: props.synced ? "#259E36" : "gray" }}
              onClick={() => {
                props.page.updateFreqWarning();
              }}
            >
              {props.synced ? "Synced" : "Out of Sync"}
            </label>
          </div>
        </li>
      </ul>
    </div>
  );
}
