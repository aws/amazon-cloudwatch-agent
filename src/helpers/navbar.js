import {Link} from "react-router-dom"
import React from 'react';
import "./nav.css"
import {FaHome,FaTable,FaChartLine, FaBookOpen} from "react-icons/fa"

//import {AiFillSetting} from 'react-icons/ai'
import logo from '../icons/inverselogo.png'
// This component handles navigation between page components.
export default function Navbar(){
    return(
        <div class="navbar">
            <ul>
                <li>
                    <img src={logo} class="logo" alt="CloudWatch logo"/>
                </li>
                <li>
                    <Link to="/"><FaHome/>Home</Link>
                </li>
                <li>
                    <Link to="/table"><FaTable/>Table</Link>
                </li>
                <li>
                    <Link to="/graphics"><FaChartLine/>Graph</Link>
                </li>
                <li>
                    <Link to="/wiki"><FaBookOpen/>Wiki</Link>
                </li>
            </ul>
        </div>
    );
}